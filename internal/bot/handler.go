package bot

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"

	"github.com/LeKSuS-04/svoi-bot/internal/db"
	"github.com/mymmrac/telego"
)

const (
	svo          string = "сво"
	zov          string = "зов"
	likvidirovan string = "ЛИКВИДИРОВАН"
)

type trigger struct {
	start int
	word  string
}

func findTriggers(text string) (triggers []trigger) {
	lowerWords := []string{svo, zov}
	lowerText := strings.ToLower(text)
	for i := range len(lowerText) {
		for _, word := range lowerWords {
			if i+len(word) <= len(lowerText) && lowerText[i:i+len(word)] == word {
				triggers = append(triggers, trigger{i, word})
			}
		}
	}
	return triggers
}

func generateResponseText() string {
	if rand.IntN(100) == 0 {
		return likvidirovan
	}
	return "Г" + strings.Repeat("О", 3+rand.IntN(10)) + "Л"
}

func generateTriggerResponseMessage(trigger trigger, responseText string, msg *telego.Message) *telego.SendMessageParams {
	return &telego.SendMessageParams{
		Text:   responseText,
		ChatID: msg.Chat.ChatID(),
		ReplyParameters: &telego.ReplyParameters{
			MessageID:     msg.MessageID,
			Quote:         msg.Text[trigger.start : trigger.start+len(trigger.word)],
			QuotePosition: trigger.start,
		},
	}
}

func (w *worker) handleUpdate(ctx context.Context, update telego.Update) error {
	switch {
	case update.Message != nil:
		return w.handleMessage(ctx, update.Message)
	default:
		return nil
	}
}

func (w *worker) handleMessage(ctx context.Context, msg *telego.Message) error {
	if msg.Text == "/svoistats" {
		return w.handleStatsRequest(ctx, msg)
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
	for _, trigger := range triggers {
		switch trigger.word {
		case svo:
			stats.SvoCount += 1
		case zov:
			stats.ZovCount += 1
		}

		responseText := generateResponseText()
		if responseText == likvidirovan {
			stats.LikvidirovanCount += 1
		}

		response := generateTriggerResponseMessage(trigger, responseText, msg)
		_, err := w.api.SendMessage(response)
		if err != nil {
			return fmt.Errorf("send message: %w", err)
		}
	}

	err := db.IncreaseStats(ctx, w.db, stats)
	if err != nil {
		return fmt.Errorf("update stats: %w", err)
	}

	return nil
}

func (w *worker) handleStatsRequest(ctx context.Context, msg *telego.Message) error {
	stats, err := db.RetrieveStats(ctx, w.db, int(msg.Chat.ID))
	if err != nil {
		return fmt.Errorf("get chat stats: %w", err)
	}

	var responseLines []string
	for _, stat := range stats {
		line := fmt.Sprintf(
			"%s: %d СВО и %d ЗОВ-ов повлекли за собой %d ЛИКВИДАЦИЙ",
			stat.UserDisplayName, stat.SvoCount, stat.ZovCount, stat.LikvidirovanCount,
		)
		responseLines = append(responseLines, line)
	}
	responseText := strings.Join(responseLines, "\n\n")

	response := &telego.SendMessageParams{
		Text:   responseText,
		ChatID: msg.Chat.ChatID(),
		ReplyParameters: &telego.ReplyParameters{
			MessageID: msg.MessageID,
		},
	}
	_, err = w.api.SendMessage(response)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}

	return nil
}
