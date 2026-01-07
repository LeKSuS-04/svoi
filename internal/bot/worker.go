package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mymmrac/telego"
	"github.com/patrickmn/go-cache"
	"golang.org/x/sync/singleflight"

	"github.com/LeKSuS-04/svoi-bot/internal/ai"
	"github.com/LeKSuS-04/svoi-bot/internal/db"
	"github.com/LeKSuS-04/svoi-bot/internal/logging"
)

type worker struct {
	config         *Config
	api            *telego.Bot
	botUsername    string
	getStickerSetG *singleflight.Group
	cache          *cache.Cache
	db             *db.DB
	ai             *ai.AI
	log            *slog.Logger
	updates        <-chan telego.Update
}

func (w *worker) Work(ctx context.Context) {
	w.log.Info("Launched worker")

loop:
	for {
		select {
		case <-ctx.Done():
			break loop

		case update := <-w.updates:
			uctx := populateUpdateContext(ctx, update)
			if err := w.handleUpdate(uctx, update); err != nil {
				w.log.ErrorContext(uctx, "failed to handle update", "error", err)
			} else {
				w.log.DebugContext(uctx, "successfully handled update")
			}
		}
	}

	w.log.Info("Stopped worker")
}

func populateUpdateContext(ctx context.Context, update telego.Update) context.Context {
	updateCtx := logging.PopulateContextID(ctx, "updateId")

	telegramAttrs := []any{
		slog.Int("updateId", update.UpdateID),
	}
	if update.Message != nil {
		telegramAttrs = append(telegramAttrs,
			slog.Int("messageId", update.Message.MessageID),
			slog.Int64("chatId", update.Message.Chat.ID),
			slog.Int64("fromId", update.Message.From.ID),
			slog.String("fromUsername", update.Message.From.Username),
		)
	}
	updateCtx = logging.PopulateContext(updateCtx, slog.Group("telegram", telegramAttrs...))

	return updateCtx
}

func (w *worker) handleUpdate(ctx context.Context, update telego.Update) (err error) {
	start := time.Now()
	defer func() {
		chatID := ""
		updateType := ""
		duration := time.Since(start).Seconds()
		if update.Message != nil {
			chatID = strconv.FormatInt(update.Message.Chat.ID, 10)
			updateType = "message"
		}

		if updateType == "" {
			return
		}

		if err != nil {
			failedUpdates.WithLabelValues(chatID, updateType).Inc()
		} else {
			successfulUpdates.WithLabelValues(chatID, updateType).Inc()
		}
		updateDurationSeconds.WithLabelValues(chatID, updateType).Observe(duration)
	}()

	switch {
	case update.Message != nil:
		return w.handleMessage(ctx, update.Message)
	default:
		return nil
	}
}

func (w *worker) handleMessage(ctx context.Context, msg *telego.Message) error {
	w.log.DebugContext(ctx, "handling message", "content", msg.Text)

	commands := []Command{
		{
			Name:    "svoistats",
			Handler: w.handleStatsRequest,
		},
		{
			Name:    "pwd",
			Handler: w.handlePwdRequest,
		},
		{
			Name:      "broadcast",
			Handler:   w.handleBroadcastRequest,
			AdminOnly: true,
		},
	}

	for _, command := range commands {
		if command.Called(msg, w.botUsername) {
			w.log.DebugContext(ctx, "running command", "command", "/"+command.Name)
			commandUsageStatistics.
				WithLabelValues(strconv.FormatInt(msg.Chat.ID, 10), command.Name).
				Inc()
			return w.RunCommand(ctx, command, msg)
		}
	}

	return w.handleRegularMessage(ctx, msg)
}

func (w *worker) handleRegularMessage(ctx context.Context, msg *telego.Message) error {
	userDisplayedName := fmt.Sprintf("%s %s", msg.From.FirstName, msg.From.LastName)
	userDisplayedName = strings.Trim(userDisplayedName, " ")
	stats := db.NamedStats{
		UserID:          int(msg.From.ID),
		ChatID:          int(msg.Chat.ID),
		UserDisplayName: userDisplayedName,
	}

	triggers := findTriggers(msg.Text)
	if len(triggers) == 0 {
		return nil
	}
	w.log.DebugContext(ctx, "found triggers", "triggers", triggers)

	if isSpam, err := w.preventSpam(ctx, msg, triggers); err != nil {
		return fmt.Errorf("prevent spam: %w", err)
	} else if isSpam {
		return nil
	}

	replies, err := w.makeReplies(ctx, msg, triggers, &stats)
	if err != nil {
		return fmt.Errorf("make responses: %w", err)
	}

	if err = w.sendReplies(msg, replies); err != nil {
		return fmt.Errorf("send replies: %w", err)
	}

	if err = w.db.IncreaseStats(ctx, stats); err != nil {
		return fmt.Errorf("update stats: %w", err)
	}

	return nil
}

func (w *worker) preventSpam(ctx context.Context, msg *telego.Message, triggers []trigger) (bool, error) {
	triggerCount := len(triggers)
	triggersLength := 0
	for _, trigger := range triggers {
		triggersLength += trigger.runeLength
	}
	textLength := utf8.RuneCountInString(msg.Text)
	spam := tooManyTriggers(triggerCount, triggersLength, textLength)

	w.log.DebugContext(ctx, "checking for spam",
		slog.Int("triggerCount", triggerCount),
		slog.Int("triggersLength", triggersLength),
		slog.Int("textLength", textLength),
		slog.Bool("isSpam", spam),
	)

	if spam {
		response := simpleReply("Спамер", msg)
		_, err := w.api.SendMessage(response)
		if err != nil {
			return false, fmt.Errorf("send message: %w", err)
		}
		return true, nil
	}
	return false, nil
}

type reply struct {
	response triggerResponse
	trigger  trigger
}

func (w *worker) makeReplies(ctx context.Context, msg *telego.Message, triggers []trigger, stats *db.NamedStats) ([]reply, error) {
	replies := make([]reply, 0, len(triggers))
	for _, trigger := range triggers {
		triggerTypeStatistics.
			WithLabelValues(chatIdLabel(msg), string(trigger.typ)).
			Inc()
		switch trigger.typ {
		case svo:
			stats.SvoCount += 1
		case zov:
			stats.ZovCount += 1
		}

		rsp, err := w.generateTriggerResponse(ctx, trigger, msg)
		if err != nil {
			w.log.ErrorContext(ctx, "failed to generate response", "error", err, "trigger", trigger, "msg", msg)
			rsp = w.makeDefaultResponse(trigger)
		}

		// Only answer with AI-generated responses
		if rsp.responseType() == aiGenerated {
			return []reply{{
				response: rsp,
				trigger:  trigger,
			}}, nil
		}

		replies = append(replies, reply{
			response: rsp,
			trigger:  trigger,
		})
	}

	// Count likvidirovan responses
	// This is done after all replies are generated, as some "likvidirovan" responses might
	// be discarded by an AI response.
	for _, r := range replies {
		if r.response.responseType() == likvidirovan {
			stats.LikvidirovanCount += 1
		}
	}

	return replies, nil
}

func (w *worker) sendReplies(msg *telego.Message, replies []reply) error {
	for _, r := range replies {
		responseTypeStatistics.
			WithLabelValues(chatIdLabel(msg), string(r.response.responseType())).
			Inc()

		err := r.response.sendReply(
			w.api,
			msg.Chat.ChatID(),
			&telego.ReplyParameters{
				MessageID:     msg.MessageID,
				Quote:         r.trigger.quote,
				QuotePosition: r.trigger.position,
			},
		)
		if err != nil {
			return fmt.Errorf("respond: %w", err)
		}
	}
	return nil
}

func chatIdLabel(msg *telego.Message) string {
	return strconv.FormatInt(msg.Chat.ID, 10)
}

func simpleReply(responseText string, msg *telego.Message) *telego.SendMessageParams {
	return &telego.SendMessageParams{
		Text:   responseText,
		ChatID: msg.Chat.ChatID(),
		ReplyParameters: &telego.ReplyParameters{
			MessageID: msg.MessageID,
		},
	}
}
