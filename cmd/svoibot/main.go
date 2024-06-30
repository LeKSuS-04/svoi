package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/sirupsen/logrus"

	"github.com/LeKSuS-04/svoi-bot/internal/bot"
)

const (
	BotTokenEnvKey     = "BOT_TOKEN"
	DebugEnvKey        = "DEBUG"
	SqliteDBPathEnvKey = "SQLITE_PATH"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	bot, err := createBot()
	if err != nil {
		logrus.WithError(err).Fatal("Failed to create bot")
	}

	err = bot.Run(ctx)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to run")
	}
}

func createBot() (*bot.Bot, error) {
	token, ok := os.LookupEnv(BotTokenEnvKey)
	if !ok {
		return nil, fmt.Errorf("no %q in env vars", BotTokenEnvKey)
	}

	var opts []bot.Option

	logger := logrus.New()
	if _, ok := os.LookupEnv(DebugEnvKey); ok {
		logger.Level = logrus.DebugLevel
	}
	opts = append(opts, bot.WithLogger(logger))

	if dbPath, ok := os.LookupEnv(SqliteDBPathEnvKey); ok {
		opts = append(opts, bot.WithDBPath(dbPath))
	}

	bot, err := bot.NewBot(token, opts...)
	if err != nil {
		return nil, fmt.Errorf("create new bot: %w", err)
	}
	return bot, nil
}
