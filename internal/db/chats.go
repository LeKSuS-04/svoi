package db

import (
	"context"
	"fmt"
)

func (db *DB) GetAllChats(ctx context.Context) (chatIDs []int, _ error) {
	chats, err := db.GetAllChats(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all chats: %w", err)
	}

	chatIDs = make([]int, 0, len(chats))
	for _, chatID := range chats {
		chatIDs = append(chatIDs, int(chatID))
	}
	return chatIDs, nil
}
