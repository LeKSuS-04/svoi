package main

import (
	"context"
	"flag"
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

	var configPath string
	flag.StringVar(&configPath, "config", "", "Path to config file")
	flag.Parse()

	bot, err := createBot(configPath)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to create bot")
	}

	err = bot.Run(ctx)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to run")
	}
}

func createBot(configPath string) (*bot.Bot, error) {
	config, err := bot.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	fmt.Printf("%+v\b", config)

	var opts []bot.Option

	logger := logrus.New()
	if config.Debug {
		logger.Level = logrus.DebugLevel
	}
	opts = append(opts, bot.WithLogger(logger), bot.WithWorkerCount(16))

	if config.SqlitePath != "" {
		opts = append(opts, bot.WithDBPath(config.SqlitePath))
	}

	bot, err := bot.NewBot(config, opts...)
	if err != nil {
		return nil, fmt.Errorf("create new bot: %w", err)
	}
	return bot, nil
}
