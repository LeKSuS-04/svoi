package bot

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"unicode/utf8"

	"github.com/LeKSuS-04/svoi-bot/internal/db"
	"github.com/mymmrac/telego"
	"github.com/sirupsen/logrus"
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

func IsTooManyTriggers(triggerCount int, triggersLength int, textLength int) bool {
	moreTriggersThan := func(maxTriggersPerMessage int) bool {
		return triggerCount > maxTriggersPerMessage
	}
	toBigTriggersLengthTimes := func(coef float64) bool {
		return coef*float64(triggersLength) > float64(textLength)
	}

	switch {
	case 0 < textLength && textLength <= 10:
		return moreTriggersThan(1)
	case 10 < textLength && textLength <= 20:
		return moreTriggersThan(2)
	case 20 < textLength && textLength <= 30:
		return moreTriggersThan(3)
	case 30 < textLength && textLength <= 50:
		return toBigTriggersLengthTimes(2.3)
	case 50 < textLength && textLength <= 100:
		return moreTriggersThan(7) && toBigTriggersLengthTimes(3.3)
	case 100 < textLength && textLength <= 250:
		return moreTriggersThan(10) && toBigTriggersLengthTimes(5.3)
	case 250 < textLength && textLength <= 1000:
		return moreTriggersThan(15) && toBigTriggersLengthTimes(11)
	default:
		return moreTriggersThan(30) && toBigTriggersLengthTimes(25)
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
	{
		triggerCount := len(triggers)
		triggersLength := 0
		for _, trigger := range triggers {
			triggersLength += utf8.RuneCountInString(trigger.word)
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

		response := replyWithQuote(responseText, trigger, msg)
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
		if stat.SvoCount + stat.ZovCount > 0 {
			line := fmt.Sprintf(
				"%s: %d СВО и %d ЗОВ-ов повлекли за собой %d ЛИКВИДАЦИЙ",
				stat.UserDisplayName, stat.SvoCount, stat.ZovCount, stat.LikvidirovanCount,
			)
			responseLines = append(responseLines, line)
		}
	}
	responseText := strings.Join(responseLines, "\n\n")

	response := simpleReply(responseText, msg)
	_, err = w.api.SendMessage(response)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
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

func replyWithQuote(responseText string, trigger trigger, msg *telego.Message) *telego.SendMessageParams {
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
