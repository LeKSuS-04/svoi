package bot

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LeKSuS-04/svoi-bot/internal/db"
	"github.com/mymmrac/telego"
	"github.com/sirupsen/logrus"
)

type Command struct {
	Name      string
	Handler   func(ctx context.Context, msg *telego.Message) error
	AdminOnly bool
}

func (c *Command) Called(msg *telego.Message, username string) bool {
	return msg.Text == fmt.Sprintf("/%s@%s", c.Name, username) ||
		msg.Text == fmt.Sprintf("/%s", c.Name)
}

func (w *worker) RunCommand(ctx context.Context, cmd Command, msg *telego.Message) error {
	if cmd.AdminOnly && !w.config.IsAdmin(msg.Chat.ID) {
		w.log.WithFields(logrus.Fields{
			"user_id":  msg.From.ID,
			"username": msg.From.Username,
			"command":  cmd.Name,
		}).Debug("User tried to execute admin command")

		response := simpleReply("You are not authorized to use this command", msg)
		_, err := w.api.SendMessage(response)
		if err != nil {
			return fmt.Errorf("send message: %w", err)
		}
		return nil
	}

	return cmd.Handler(ctx, msg)
}

func (w *worker) handleStatsRequest(ctx context.Context, msg *telego.Message) error {
	stats, err := w.db.RetrieveStats(ctx, int(msg.Chat.ID))
	if err != nil {
		return fmt.Errorf("get chat stats: %w", err)
	}

	responseLines := make([]string, 0, len(stats))
	for _, stat := range stats {
		line := fmtStatsLine(stat)
		responseLines = append(responseLines, line)
	}
	responseText := strings.Join(responseLines, "\n\n")

	response := simpleReply(responseText, msg)
	_, err = w.api.SendMessage(response)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}

	return nil
}

func fmtStatsLine(stat db.NamedStats) string {
	svoStr := "СВО"

	zovStr := func() string {
		switch stat.ZovCount % 10 {
		case 1:
			return "ЗОВ"
		case 2, 3, 4:
			return "ЗОВ-а"
		default:
			return "ЗОВ-ов"
		}
	}()

	likvidirovanStr := func() string {
		switch stat.LikvidirovanCount {
		case 1:
			return "ЛИКВИДАЦИЮ"
		case 2, 3, 4:
			return "ЛИКВИДАЦИИ"
		default:
			return "ЛИКВИДАЦИЙ"
		}
	}()

	return fmt.Sprintf(
		"%s: %d %s и %d %s повлекли за собой %d %s",
		stat.UserDisplayName,
		stat.SvoCount, svoStr,
		stat.ZovCount, zovStr,
		stat.LikvidirovanCount, likvidirovanStr,
	)
}

func (w *worker) handlePwdRequest(ctx context.Context, msg *telego.Message) error {
	text := fmt.Sprintf("chat_id: %d", msg.Chat.ID)
	if w.config.IsAdmin(msg.Chat.ID) {
		text += "\nis_admin: true"
	}

	response := simpleReply(text, msg)
	_, err := w.api.SendMessage(response)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}

	return nil
}

func (w *worker) handleBroadcastRequest(ctx context.Context, msg *telego.Message) error {
	if msg.ReplyToMessage == nil {
		response := simpleReply("Please reply to a message to broadcast it", msg)
		_, err := w.api.SendMessage(response)
		if err != nil {
			return fmt.Errorf("send message: %w", err)
		}
		return nil
	}

	broadcastText := msg.ReplyToMessage.Text
	if broadcastText == "" {
		response := simpleReply("Please reply to a message with text to broadcast it", msg)
		_, err := w.api.SendMessage(response)
		if err != nil {
			return fmt.Errorf("send message: %w", err)
		}
		return nil
	}

	chats, err := w.db.GetAllChats(ctx)
	if err != nil {
		return fmt.Errorf("get all chats: %w", err)
	}
	w.log.WithFields(logrus.Fields{
		"chats":   chats,
		"message": msg.ReplyToMessage.Text,
	}).Debug("Broadcasting message to chats")

	errs := make([]error, 0)
	successCount := 0
	failureCount := 0
	for _, chatID := range chats {
		if chatID == int(msg.Chat.ID) {
			continue
		}

		_, err := w.api.SendMessage(&telego.SendMessageParams{
			ChatID: telego.ChatID{ID: int64(chatID)},
			Text:   broadcastText,
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("send message to chat %d: %w", chatID, err))
			failureCount++
		} else {
			successCount++
		}
	}
	if sendErr := errors.Join(errs...); sendErr != nil {
		w.log.WithError(sendErr).Warn("Failed to send message to some chats")
	}

	response := simpleReply(
		fmt.Sprintf(
			"Finished broadcasting to %d chats: %d success, %d failure",
			len(chats)-1, successCount, failureCount,
		),
		msg,
	)
	_, err = w.api.SendMessage(response)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}

	return nil
}
