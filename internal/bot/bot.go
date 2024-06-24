package bot

import (
	"fmt"
	"time"

	"github.com/LeKSuS-04/svoi-bot/internal/db"
	"github.com/mymmrac/telego"
	"github.com/sirupsen/logrus"
)

type Bot struct {
	workerCount          int
	cacheDuration        time.Duration
	cacheCleanupInterval time.Duration
	dbPath               string

	api *telego.Bot
	log *logrus.Logger
}

type Option = func(*Bot)

func NewBot(token string, opts ...Option) (*Bot, error) {
	api, err := telego.NewBot(token)
	if err != nil {
		return nil, fmt.Errorf("create new bot api: %w", err)
	}

	b := &Bot{
		workerCount:          4,
		cacheDuration:        time.Hour * 1,
		cacheCleanupInterval: time.Minute * 5,
		dbPath:               db.InMemory,

		api: api,
		log: logrus.StandardLogger(),
	}

	for _, opt := range opts {
		opt(b)
	}
	return b, nil
}

func WithLogger(log *logrus.Logger) Option {
	return func(b *Bot) {
		b.log = log
	}
}

func WithWorkerCount(workers int) Option {
	return func(b *Bot) {
		b.workerCount = workers
	}
}

func WithCacheDuration(duration time.Duration) Option {
	return func(b *Bot) {
		b.cacheDuration = duration
	}
}

func WithCacheCleanupInterval(duration time.Duration) Option {
	return func(b *Bot) {
		b.cacheCleanupInterval = duration
	}
}

func WithDBPath(dbPath string) Option {
	return func(b *Bot) {
		b.dbPath = dbPath
	}
}
