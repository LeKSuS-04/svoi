package bot

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"

	"github.com/mymmrac/telego"
)

type position struct {
	start  int
	length int
}

func findTriggers(text string) (triggers []position) {
	lowerWords := []string{"сво", "зов"}
	lowerText := strings.ToLower(text)
	for i := range len(lowerText) {
		for _, word := range lowerWords {
			if i+len(word) <= len(lowerText) && lowerText[i:i+len(word)] == word {
				triggers = append(triggers, position{i, len(word)})
			}
		}
	}
	return triggers
}

func generateResponseText() string {
	if rand.IntN(100) == 0 {
		return "ЛИКВИДИРОВАН"
	}
	return "Г" + strings.Repeat("О", 3+rand.IntN(10)) + "Л"
}

func generateResponseMessage(trigger position, msg *telego.Message) *telego.SendMessageParams {
	return &telego.SendMessageParams{
		Text:   generateResponseText(),
		ChatID: msg.Chat.ChatID(),
		ReplyParameters: &telego.ReplyParameters{
			MessageID:     msg.MessageID,
			Quote:         msg.Text[trigger.start : trigger.start+trigger.length],
			QuotePosition: trigger.start,
		},
	}
}

func (w *worker) handleUpdate(update telego.Update, ctx context.Context) error {
	switch {
	case update.Message != nil:
		return w.handleMessage(update.Message, ctx)
	default:
		return nil
	}
}

func (w *worker) handleMessage(msg *telego.Message, ctx context.Context) error {
	triggers := findTriggers(msg.Text)
	for _, trigger := range triggers {
		response := generateResponseMessage(trigger, msg)
		_, err := w.api.SendMessage(response)
		if err != nil {
			return fmt.Errorf("send message: %w", err)
		}
	}
	return nil
}
