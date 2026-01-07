package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/LeKSuS-04/svoi-bot/internal/logging"

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

	log := logging.New("setup")

	bot, err := createBot(configPath)
	if err != nil {
		log.ErrorContext(ctx, "failed to create bot", "error", err)
		os.Exit(1)
	}

	err = bot.Run(ctx)
	if err != nil {
		log.ErrorContext(ctx, "failed to run bot", "error", err)
		os.Exit(1)
	}
}

func createBot(configPath string) (*bot.Bot, error) {
	config, err := bot.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	var opts []bot.Option
	opts = append(opts, bot.WithWorkerCount(16))

	if config.SqlitePath != "" {
		opts = append(opts, bot.WithDBPath(config.SqlitePath))
	}

	bot, err := bot.NewBot(config, opts...)
	if err != nil {
		return nil, fmt.Errorf("create new bot: %w", err)
	}
	return bot, nil
}
