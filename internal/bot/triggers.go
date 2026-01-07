package bot

import (
	"context"
	"fmt"
	"math/rand/v2"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/mymmrac/telego"
)

type triggerType string
type responseType string

const (
	svo triggerType = "svo"
	zov triggerType = "zov"

	regular      responseType = "regular"
	likvidirovan responseType = "likvidirovan"
	aiGenerated  responseType = "ai_generated"

	svoRegexp = "[сСsScC][вВvVB8][оОoO0]+"
	zovRegexp = "[зЗzZ3][оОoO0]+[8вВvVB]"
)

type matcher struct {
	typ triggerType
	re  *regexp.Regexp
}

var matchers = []matcher{
	{
		typ: svo,
		re:  regexp.MustCompile(svoRegexp),
	},
	{
		typ: zov,
		re:  regexp.MustCompile(zovRegexp),
	},
}

type trigger struct {
	position int
	quote    string

	runeLength int
	typ        triggerType
}

type triggerResponse interface {
	trigger() trigger
	responseType() responseType
	sendReply(api *telego.Bot, chatID telego.ChatID, replyParams *telego.ReplyParameters) error
}

type triggerResponseBase struct {
	t   trigger
	typ responseType
}

func (t *triggerResponseBase) trigger() trigger {
	return t.t
}

func (t *triggerResponseBase) responseType() responseType {
	return t.typ
}

type textResponse struct {
	triggerResponseBase
	text string
}

func (t *textResponse) sendReply(api *telego.Bot, chatID telego.ChatID, replyParams *telego.ReplyParameters) error {
	_, err := api.SendMessage(
		&telego.SendMessageParams{
			Text:            t.text,
			ChatID:          chatID,
			ReplyParameters: replyParams,
		},
	)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	return nil
}

type stickerResponse struct {
	triggerResponseBase
	fileID string
}

func (s *stickerResponse) sendReply(api *telego.Bot, chatID telego.ChatID, replyParams *telego.ReplyParameters) error {
	_, err := api.SendSticker(
		&telego.SendStickerParams{
			Sticker: telego.InputFile{
				FileID: s.fileID,
			},
			ChatID:          chatID,
			ReplyParameters: replyParams,
		},
	)
	if err != nil {
		return fmt.Errorf("send sticker: %w", err)
	}
	return nil
}

func (w *worker) generateTriggerResponse(ctx context.Context, trigger trigger, msg *telego.Message) (triggerResponse, error) {
	rng := rand.IntN(100)

	allowAI := true
	if w.ai == nil {
		allowAI = false
	} else if _, ok := w.cache.Get(aiSenderKey(msg.From.ID)); ok {
		allowAI = false
	}

	switch {
	case rng == 0:
		return w.makeLikvidirovanResponse(trigger), nil

	case rng < 20:
		return w.makeStickerResponse(trigger), nil

	case rng >= 60 && allowAI:
		resp, err := w.makeAIResponse(ctx, trigger, msg)
		if err != nil {
			return nil, fmt.Errorf("make ai response: %w", err)
		}
		return resp, nil

	default:
		return w.makeDefaultResponse(trigger), nil
	}
}

func (w *worker) makeDefaultResponse(trigger trigger) triggerResponse {
	return &textResponse{
		triggerResponseBase: triggerResponseBase{
			t: trigger, typ: regular,
		},
		text: "Г" + strings.Repeat("О", 3+rand.IntN(10)) + "Л",
	}
}

func (w *worker) makeLikvidirovanResponse(trigger trigger) triggerResponse {
	return &textResponse{
		triggerResponseBase: triggerResponseBase{
			t: trigger, typ: likvidirovan,
		},
		text: "ЛИКВИДИРОВАН",
	}
}

func (w *worker) makeStickerResponse(trigger trigger) triggerResponse {
	fileID, err := w.getSticker()
	if err != nil {
		return w.makeDefaultResponse(trigger)
	}
	return &stickerResponse{
		triggerResponseBase: triggerResponseBase{
			t: trigger, typ: regular,
		},
		fileID: fileID,
	}
}

func (w *worker) makeAIResponse(ctx context.Context, trigger trigger, msg *telego.Message) (triggerResponse, error) {
	if w.ai == nil {
		return w.makeDefaultResponse(trigger), nil
	}

	if _, ok := w.cache.Get(aiSenderKey(msg.From.ID)); ok {
		return w.makeDefaultResponse(trigger), nil
	}

	if !isAIRespondable(msg.Text) {
		return w.makeDefaultResponse(trigger), nil
	}

	w.log.InfoContext(ctx, "generating ai response", "text", msg.Text)

	if err := w.cache.Add(aiSenderKey(msg.From.ID), struct{}{}, w.config.AI.ResponseResetPeriod); err != nil {
		w.log.ErrorContext(ctx, "failed to add to cache", "error", err)
		return w.makeDefaultResponse(trigger), nil
	}

	resp, err := w.ai.GeneratePatrioticResponse(ctx, msg.Text)
	if err != nil {
		w.cache.Delete(aiSenderKey(msg.From.ID))
		return nil, fmt.Errorf("generate patriotic response: %w", err)
	}

	return &textResponse{
		triggerResponseBase: triggerResponseBase{
			t: trigger, typ: aiGenerated,
		},
		text: resp,
	}, nil
}

func isAIRespondable(msg string) bool {
	if msg == "" {
		return false
	}

	words := strings.Fields(msg)
	if len(words) < 5 {
		return false
	}

	exactSvoPattern := regexp.MustCompile("^" + svoRegexp + "$")
	svoPattern := regexp.MustCompile(svoRegexp)
	exactZovPattern := regexp.MustCompile("^" + zovRegexp + "$")
	zovPattern := regexp.MustCompile(zovRegexp)

	patternCount := 0
	containsPatternCount := 0
	normalWordCount := 0

	for _, word := range words {
		lowercaseWord := strings.ToLower(word)

		if exactSvoPattern.MatchString(lowercaseWord) ||
			exactZovPattern.MatchString(lowercaseWord) {
			patternCount++
		} else if svoPattern.MatchString(lowercaseWord) ||
			zovPattern.MatchString(lowercaseWord) {
			containsPatternCount++
		} else {
			normalWordCount++
		}
	}

	return normalWordCount >= 5 && patternCount <= 2 && containsPatternCount <= 2
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
				typ:        matcher.typ,
			})
		}
	}
	return triggers
}

func tooManyTriggers(triggerCount, triggersLength, textLength int) bool {
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

func aiSenderKey(senderID int64) string {
	return fmt.Sprintf("ai:reset_period:%d", senderID)
}
