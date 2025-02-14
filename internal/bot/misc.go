package bot

import (
	"fmt"
	"math/rand/v2"
	"slices"

	"github.com/mymmrac/telego"
)

func stickerSetCacheKey(stickerSetName string) string {
	return "sticker_set:" + stickerSetName
}

func (w *worker) getSticker() (stickerFileID string, err error) {
	stickerSetConfig := w.config.StickerSets[rand.IntN(len(w.config.StickerSets))]
	key := stickerSetCacheKey(stickerSetConfig.Name)
	set, ok := w.cache.Get(key)
	if !ok {
		set, err = w.loadStickerSet(stickerSetConfig)
		if err != nil {
			return "", fmt.Errorf("load sticker set: %w", err)
		}
		w.cache.Set(key, set, 0)
	}

	setStr := set.([]string)
	stickerFileID = setStr[rand.IntN(len(setStr))]
	return stickerFileID, nil
}

func (w *worker) loadStickerSet(stickerSetConfig StickerSetConfig) (stickerFileIDs []string, _ error) {
	stickerSet, err := w.api.GetStickerSet(&telego.GetStickerSetParams{
		Name: stickerSetConfig.Name,
	})
	if err != nil {
		return nil, fmt.Errorf("get sticker set: %w", err)
	}

	for _, sticker := range stickerSet.Stickers {
		if !slices.Contains(stickerSetConfig.ExcludeStickerIDs, sticker.FileID) {
			stickerFileIDs = append(stickerFileIDs, sticker.FileID)
		}
	}
	return stickerFileIDs, nil
}

const selfUsernameKey = "self_username"

func (w *worker) getSelfUsername() (string, error) {
	username, ok := w.cache.Get(selfUsernameKey)
	if ok {
		return username.(string), nil
	}

	self, err := w.api.GetMe()
	if err != nil {
		return "", fmt.Errorf("get self: %w", err)
	}

	w.cache.Set(selfUsernameKey, self.Username, -1)
	return self.Username, nil
}
