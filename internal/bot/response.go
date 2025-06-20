package bot

import (
	"context"
	"fmt"
	"math/rand/v2"
	"regexp"
	"strings"

	"github.com/mymmrac/telego"
)

func aiSenderKey(senderID int64) string {
	return fmt.Sprintf("ai:reset_period:%d", senderID)
}

const (
	regular      responseType = "regular"
	likvidirovan responseType = "likvidirovan"
	aiGenerated  responseType = "ai_generated"
)

type response interface {
	getType() responseType
	reply(api *telego.Bot, chatID telego.ChatID, replyParams *telego.ReplyParameters) error
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

func (w *worker) generateResponse(ctx context.Context, msg *telego.Message) (response, error) {
	rng := rand.IntN(100)

	allowAI := true
	if w.ai == nil {
		allowAI = false
	} else if _, ok := w.cache.Get(aiSenderKey(msg.From.ID)); ok {
		allowAI = false
	}

	makeDefaultResponse := func() *textResponse {
		return &textResponse{
			text:  "Г" + strings.Repeat("О", 3+rand.IntN(10)) + "Л",
			ttype: regular,
		}
	}

	switch {
	case rng == 0:
		return &textResponse{
			text:  "ЛИКВИДИРОВАН",
			ttype: likvidirovan,
		}, nil

	case rng < 20:
		fileID, err := w.getSticker()
		if err != nil {
			return nil, fmt.Errorf("get sticker: %w", err)
		}
		return &stickerResponse{
			fileID: fileID,
			ttype:  regular,
		}, nil

	case rng >= 60 && allowAI && isAIRespondable(msg.Text):
		log := w.log.WithField("sender_id", msg.From.ID)
		log.WithField("text", msg.Text).Info("Generating AI response")

		if err := w.cache.Add(aiSenderKey(msg.From.ID), struct{}{}, w.config.AI.ResponseResetPeriod); err != nil {
			log.WithError(err).Debug("Failed to add to cache")
			return makeDefaultResponse(), nil
		}

		resp, err := w.ai.GeneratePatrioticResponse(ctx, log, msg.Text)
		if err != nil {
			w.cache.Delete(aiSenderKey(msg.From.ID))
			return nil, fmt.Errorf("generate ai response: %w", err)
		}

		log.WithField("response", resp).Info("Generated AI response")
		return &textResponse{text: resp, ttype: aiGenerated}, nil

	default:
		return makeDefaultResponse(), nil
	}
}

type textResponse struct {
	text  string
	ttype responseType
}

func (t *textResponse) getType() responseType {
	return t.ttype
}

func (t *textResponse) reply(api *telego.Bot, chatID telego.ChatID, replyParams *telego.ReplyParameters) error {
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
	fileID string
	ttype  responseType
}

func (s *stickerResponse) getType() responseType {
	return s.ttype
}

func (s *stickerResponse) reply(api *telego.Bot, chatID telego.ChatID, replyParams *telego.ReplyParameters) error {
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
