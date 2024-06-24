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

func IncreaseStats(ctx context.Context, db *DB, stats NamedStats) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	displayedName, err := db.WithTx(tx).GetUser(ctx, int64(stats.UserID))
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("get user %d: %w", stats.UserID, err)
	}

	if err == sql.ErrNoRows {
		err := db.WithTx(tx).CreateUser(ctx, q.CreateUserParams{
			ID:            int64(stats.UserID),
			DisplayedName: stats.UserDisplayName,
		})
		if err != nil {
			return fmt.Errorf("create user: %w", err)
		}
	} else if displayedName != stats.UserDisplayName {
		err := db.WithTx(tx).UpdateUser(ctx, q.UpdateUserParams{
			ID:            int64(stats.UserID),
			DisplayedName: stats.UserDisplayName,
		})
		if err != nil {
			return fmt.Errorf("update displayed name: %w", err)
		}
	}

	err = db.WithTx(tx).InitStat(ctx, q.InitStatParams{
		UserID: int64(stats.UserID),
		ChatID: int64(stats.ChatID),
	})
	if err != nil {
		return fmt.Errorf("init stat: %w", err)
	}

	err = db.WithTx(tx).AddStats(ctx, q.AddStatsParams{
		UserID:            int64(stats.UserID),
		ChatID:            int64(stats.ChatID),
		SvoCount:          int64(stats.SvoCount),
		ZovCount:          int64(stats.ZovCount),
		LikvidirovanCount: int64(stats.LikvidirovanCount),
	})
	if err != nil {
		return fmt.Errorf("add stats: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit changes: %w", err)
	}

	return nil
}

func RetrieveStats(ctx context.Context, db *DB, chatID int) ([]NamedStats, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stats, err := db.GetChatStats(ctx, int64(chatID))
	if err != nil {
		return nil, fmt.Errorf("get chat stats: %w", err)
	}

	var namedStats []NamedStats
	for _, stat := range stats {
		displayedName, err := db.GetUser(ctx, stat.UserID)
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

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit changes: %w", err)
	}

	return namedStats, nil
}
