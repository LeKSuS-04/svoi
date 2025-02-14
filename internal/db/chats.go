package db

import (
	"context"
	"fmt"
)

func GetAllChats(ctx context.Context, connector Connector) (chatIDs []int, _ error) {
	db, err := connector.Connect()
	if err != nil {
		return nil, fmt.Errorf("connect to db: %w", err)
	}

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
