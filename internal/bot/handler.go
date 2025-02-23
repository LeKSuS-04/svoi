package bot

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/mymmrac/telego"
	"github.com/sirupsen/logrus"

	"github.com/LeKSuS-04/svoi-bot/internal/db"
)

func (w *worker) handleUpdate(ctx context.Context, update telego.Update) error {
	switch {
	case update.Message != nil:
		return w.handleMessage(ctx, update.Message)
	default:
		return nil
	}
}

func (w *worker) handleMessage(ctx context.Context, msg *telego.Message) error {
	username, err := w.getSelfUsername()
	if err != nil {
		return fmt.Errorf("get self username: %w", err)
	}

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
		if command.Called(msg, username) {
			w.log.WithField("command", "/"+command.Name).Debug("Running command")
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
		if IsTooManyTriggers(triggerCount, triggersLength, textLength) {
			response := simpleReply("Спамер", msg)
			_, err := w.api.SendMessage(response)
			if err != nil {
				return fmt.Errorf("send message: %w", err)
			}
			return nil
		}
	}

	for _, trigger := range triggers {
		switch trigger.ttype {
		case svo:
			stats.SvoCount += 1
		case zov:
			stats.ZovCount += 1
		}

		rsp, err := w.generateResponse()
		if err != nil {
			return fmt.Errorf("generate response: %w", err)
		}

		if rsp.getType() == likvidirovan {
			stats.LikvidirovanCount += 1
		}

		err = rsp.reply(
			w.api,
			msg.Chat.ChatID(),
			&telego.ReplyParameters{
				MessageID:     msg.MessageID,
				Quote:         trigger.quote,
				QuotePosition: trigger.position,
			},
		)
		if err != nil {
			return fmt.Errorf("respond: %w", err)
		}
	}

	err := db.IncreaseStats(ctx, w.connector, stats)
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
