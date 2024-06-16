package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/LeKSuS-04/svoi-bot/internal/bot"
	"github.com/sirupsen/logrus"
)

const (
	BotTokenEnvKey = "BOT_TOKEN"
	DebugEnvKey    = "DEBUG"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	bot, err := createBot()
	if err != nil {
		logrus.WithError(err).Panicf("Failed to create bot")
	}

	bot.Run(ctx)
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

	bot, err := bot.NewBot(token, opts...)
	if err != nil {
		return nil, fmt.Errorf("create new bot: %w", err)
	}
	return bot, nil
}
