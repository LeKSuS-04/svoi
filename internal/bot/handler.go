package bot

import (
	"context"
	"fmt"
	"math/rand/v2"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/mymmrac/telego"
	"github.com/sirupsen/logrus"

	"github.com/LeKSuS-04/svoi-bot/internal/db"
)

type triggerType int
type responseType int

const (
	svo triggerType = iota
	zov triggerType = iota

	gol          responseType = iota
	likvidirovan responseType = iota
)

type matcher struct {
	ttype triggerType
	re    *regexp.Regexp
}

var matchers = []matcher{
	{
		ttype: svo,
		re:    regexp.MustCompile("[сС][вВ][оО]+"),
	},
	{
		ttype: svo,
		re:    regexp.MustCompile("[зЗ][оО]+[вВ]"),
	},
}

type trigger struct {
	position int
	quote    string

	runeLength int
	ttype      triggerType
}

type response struct {
	text  string
	ttype responseType
}

func findTriggers(text string) (triggers []trigger) {
	for _, matcher := range matchers {
		matches := matcher.re.FindAllStringIndex(text, -1)
		for _, match := range matches {
			quote := text[match[0]:match[1]]
			triggers = append(triggers, trigger{
				quote:      quote,
				position:   utf8.RuneCountInString(text[:match[0]]),
				runeLength: utf8.RuneCountInString(quote),
				ttype:      matcher.ttype,
			})
		}
	}
	return triggers
}

func generateResponseText() response {
	if rand.IntN(100) == 0 {
		return response{
			text:  "ЛИКВИДИРОВАН",
			ttype: likvidirovan,
		}
	}
	return response{
		text:  "Г" + strings.Repeat("О", 3+rand.IntN(10)) + "Л",
		ttype: gol,
	}
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
	if strings.HasPrefix(msg.Text, "/svoistats") {
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

		responseText := generateResponseText()
		if responseText.ttype == likvidirovan {
			stats.LikvidirovanCount += 1
		}

		response := replyWithQuote(responseText.text, trigger, msg)
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
		if stat.SvoCount+stat.ZovCount > 0 {
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
			Quote:         trigger.quote,
			QuotePosition: trigger.position,
		},
	}
}
