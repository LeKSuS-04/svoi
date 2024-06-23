package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/LeKSuS-04/svoi-bot/internal/db/q"
)

type NamedStats struct {
	UserID            int
	ChatID            int
	UserDisplayName   string
	ZovCount          int
	SvoCount          int
	LikvidirovanCount int
}

func IncreaseStats(ctx context.Context, queries *q.Queries, stats NamedStats) error {
	displayedName, err := queries.GetUser(ctx, int64(stats.UserID))
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("get user %d: %w", stats.UserID, err)
	}

	if err == sql.ErrNoRows {
		err := queries.CreateUser(ctx, q.CreateUserParams{
			ID:            int64(stats.UserID),
			DisplayedName: stats.UserDisplayName,
		})
		if err != nil {
			return fmt.Errorf("create user: %w", err)
		}
	} else if displayedName != stats.UserDisplayName {
		err := queries.UpdateUser(ctx, q.UpdateUserParams{
			ID:            int64(stats.UserID),
			DisplayedName: stats.UserDisplayName,
		})
		if err != nil {
			return fmt.Errorf("update displayed name: %w", err)
		}
	}

	err = queries.InitStat(ctx, q.InitStatParams{
		UserID: int64(stats.UserID),
		ChatID: int64(stats.ChatID),
	})
	if err != nil {
		return fmt.Errorf("init stat: %w", err)
	}

	err = queries.AddStats(ctx, q.AddStatsParams{
		UserID:            int64(stats.UserID),
		ChatID:            int64(stats.ChatID),
		SvoCount:          int64(stats.SvoCount),
		ZovCount:          int64(stats.ZovCount),
		LikvidirovanCount: int64(stats.LikvidirovanCount),
	})
	if err != nil {
		return fmt.Errorf("add stats: %w", err)
	}

	return nil
}

func RetrieveStats(ctx context.Context, queries *q.Queries, chatID int) ([]NamedStats, error) {
	stats, err := queries.GetChatStats(ctx, int64(chatID))
	if err != nil {
		return nil, fmt.Errorf("get chat stats: %w", err)
	}

	var namedStats []NamedStats
	for _, stat := range stats {
		displayedName, err := queries.GetUser(ctx, stat.UserID)
		if err != nil {
			return nil, fmt.Errorf("get user %d: %w", stat.UserID, err)
		}
		namedStats = append(namedStats, NamedStats{
			UserID:            int(stat.UserID),
			ChatID:            chatID,
			UserDisplayName:   displayedName,
			SvoCount:          int(stat.SvoCount),
			ZovCount:          int(stat.ZovCount),
			LikvidirovanCount: int(stat.LikvidirovanCount),
		})
	}

	return namedStats, nil
}
