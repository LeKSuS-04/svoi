package bot

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mymmrac/telego"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/singleflight"

	"github.com/LeKSuS-04/svoi-bot/internal/ai"
	"github.com/LeKSuS-04/svoi-bot/internal/db"
)

type Bot struct {
	config               *Config
	workerCount          int
	cacheDuration        time.Duration
	cacheCleanupInterval time.Duration
	dbPath               string

	api *telego.Bot
	log *logrus.Logger
}

func NewBot(config *Config, opts ...Option) (*Bot, error) {
	api, err := telego.NewBot(config.BotToken)
	if err != nil {
		return nil, fmt.Errorf("create new bot api: %w", err)
	}

	b := &Bot{
		config:               config,
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

func (b *Bot) Run(ctx context.Context) error {
	b.log.Debug("Running in debug mode")

	cache := cache.New(b.cacheDuration, b.cacheCleanupInterval)
	workerUpdatesChan := make(chan telego.Update, 1000)

	var aiHandler *ai.AI
	if b.config.AI.APIKey == "" {
		b.log.Warn("AI API key is not set, AI responses will be disabled")
	} else {
		aiHandler = ai.NewAI(b.config.AI)
	}

	db, err := db.NewDB(b.dbPath)
	if err != nil {
		return fmt.Errorf("open db connection: %w", err)
	}

	self, err := b.api.GetMe()
	if err != nil {
		return fmt.Errorf("get self: %w", err)
	}

	stickerSetG := &singleflight.Group{}

	wg := sync.WaitGroup{}
	wg.Add(b.workerCount)
	for i := range b.workerCount {
		workerId := i
		go func() {
			defer wg.Done()
			w := worker{
				config:         b.config,
				api:            b.api,
				botUsername:    self.Username,
				getStickerSetG: stickerSetG,
				cache:          cache,
				db:             db,
				ai:             aiHandler,
				log: b.log.WithFields(logrus.Fields{
					"component": "worker",
					"workerID":  workerId,
				}),
				updates: workerUpdatesChan,
			}
			w.Work(ctx)
		}()
	}

	newUpdatesChan, err := b.api.UpdatesViaLongPolling(
		nil,
		telego.WithLongPollingContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("subscribe to updates: %w", err)
	}

	if b.config.Metrics != nil && b.config.Metrics.Addr != "" {
		b.runMetricsServer(ctx)
	}

	b.log.Infof("Listening for updates from bot %q", self.Username)
loop:
	for {
		select {
		case <-ctx.Done():
			break loop

		case update := <-newUpdatesChan:
			b.log.WithField("updateId", update.UpdateID).Debug("Received new update")
			workerUpdatesChan <- update
		}
	}
	b.log.Info("Stopped receiving updates")

	b.log.Info("Waiting for workers to shut down")
	wg.Wait()
	b.log.Info("All workers stopped")
	return nil
}

type Option = func(*Bot)

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
