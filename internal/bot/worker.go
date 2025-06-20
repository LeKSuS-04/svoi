package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mymmrac/telego"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/singleflight"

	"github.com/LeKSuS-04/svoi-bot/internal/ai"
	"github.com/LeKSuS-04/svoi-bot/internal/db"
)

type worker struct {
	config         *Config
	api            *telego.Bot
	botUsername    string
	getStickerSetG *singleflight.Group
	cache          *cache.Cache
	db             *db.DB
	ai             *ai.AI
	log            *logrus.Entry
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
			err := w.handleUpdate(ctx, update)
			if err != nil {
				w.log.
					WithField("updateId", update.UpdateID).
					WithError(err).
					Error("Failed to handle update")
			} else {
				w.log.WithField("updateId", update.UpdateID).Debug("Successfully handled update")
			}
		}
	}

	w.log.Info("Stopped worker")
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
			w.log.WithField("command", "/"+command.Name).Debug("Running command")
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
	w.log.WithField("triggers", triggers).Debug("Found triggers")
	{
		triggerCount := len(triggers)
		triggersLength := 0
		for _, trigger := range triggers {
			triggersLength += trigger.runeLength
		}
		textLength := utf8.RuneCountInString(msg.Text)
		w.log.WithFields(logrus.Fields{
			"triggerCount":   triggerCount,
			"triggersLength": triggersLength,
			"textLength":     textLength,
		}).Debug("Evaluating spammer status")
		if tooManyTriggers(triggerCount, triggersLength, textLength) {
			response := simpleReply("Спамер", msg)
			_, err := w.api.SendMessage(response)
			if err != nil {
				return fmt.Errorf("send message: %w", err)
			}
			return nil
		}
	}

	type reply struct {
		response triggerResponse
		trigger  trigger
	}
	replies := make([]reply, len(triggers))
	chatIDLabelValue := strconv.FormatInt(msg.Chat.ID, 10)
	for i, trigger := range triggers {
		triggerTypeStatistics.
			WithLabelValues(chatIDLabelValue, string(trigger.typ)).
			Inc()
		switch trigger.typ {
		case svo:
			stats.SvoCount += 1
		case zov:
			stats.ZovCount += 1
		}

		rsp, err := w.generateTriggerResponse(ctx, trigger, msg)
		if err != nil {
			w.log.WithError(err).WithFields(logrus.Fields{
				"trigger": trigger,
				"msg":     msg,
			}).Warn("Failed to generate response")
			rsp = w.makeDefaultResponse(trigger)
		}

		// Only answer with AI-generated responses
		if rsp.responseType() == aiGenerated {
			replies = []reply{{
				response: rsp,
				trigger:  trigger,
			}}
			break
		}

		replies[i] = reply{
			response: rsp,
			trigger:  trigger,
		}
	}

	for _, r := range replies {
		responseTypeStatistics.
			WithLabelValues(chatIDLabelValue, string(r.response.responseType())).
			Inc()

		if r.response.responseType() == likvidirovan {
			stats.LikvidirovanCount += 1
		}

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

	err := w.db.IncreaseStats(ctx, stats)
	if err != nil {
		return fmt.Errorf("update stats: %w", err)
	}

	return nil
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
