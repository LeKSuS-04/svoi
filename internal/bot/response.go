package bot

import (
	"fmt"
	"math/rand/v2"
	"strings"

	"github.com/mymmrac/telego"
)

const (
	regular      responseType = iota
	likvidirovan responseType = iota
)

type response interface {
	getType() responseType
	reply(api *telego.Bot, chatID telego.ChatID, replyParams *telego.ReplyParameters) error
}

func (w *worker) generateResponse() (response, error) {
	rng := rand.IntN(100)

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

	default:
		return &textResponse{
			text:  "Г" + strings.Repeat("О", 3+rand.IntN(10)) + "Л",
			ttype: regular,
		}, nil
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
