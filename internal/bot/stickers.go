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

	v, err, _ := w.getStickerSetG.Do(key, func() (any, error) {
		set, ok := w.cache.Get(key)
		if ok {
			return set, nil
		}
		stickerFileIDs, err := w.loadStickerSet(stickerSetConfig)
		if err != nil {
			return nil, fmt.Errorf("load sticker set: %w", err)
		}
		w.cache.Set(key, stickerFileIDs, 0)
		return stickerFileIDs, nil
	})
	if err != nil {
		return "", fmt.Errorf("get sticker set: %w", err)
	}

	setStr := v.([]string)
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
