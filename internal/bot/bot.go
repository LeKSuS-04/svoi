package bot

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/mymmrac/telego"
	"github.com/patrickmn/go-cache"
	"golang.org/x/sync/singleflight"

	"github.com/LeKSuS-04/svoi-bot/internal/ai"
	"github.com/LeKSuS-04/svoi-bot/internal/db"
	"github.com/LeKSuS-04/svoi-bot/internal/logging"
)

type Bot struct {
	config               *Config
	workerCount          int
	cacheDuration        time.Duration
	cacheCleanupInterval time.Duration
	dbPath               string

	api *telego.Bot
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
	}

	for _, opt := range opts {
		opt(b)
	}
	return b, nil
}

func (b *Bot) Run(ctx context.Context) error {
	log := logging.New("bot")
	log.DebugContext(ctx, "running in debug mode")

	cache := cache.New(b.cacheDuration, b.cacheCleanupInterval)
	workerUpdatesChan := make(chan telego.Update, 1000)

	var aiHandler *ai.AI
	if b.config.AI.APIKey == "" {
		log.WarnContext(ctx, "AI API key is not set, AI responses will be disabled")
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
				log:            logging.New(fmt.Sprintf("worker-%d", workerId)),
				updates:        workerUpdatesChan,
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

	log.InfoContext(ctx, "listening for updates from bot", "username", self.Username)
	skipQueuedUpdates(ctx, log, newUpdatesChan)

loop:
	for {
		select {
		case <-ctx.Done():
			break loop

		case update := <-newUpdatesChan:
			log.DebugContext(ctx, "received new update", "updateId", update.UpdateID)
			workerUpdatesChan <- update
		}
	}
	log.InfoContext(ctx, "stopped receiving updates")

	log.InfoContext(ctx, "waiting for workers to shut down")
	wg.Wait()
	log.InfoContext(ctx, "all workers stopped")
	return nil
}

func skipQueuedUpdates(ctx context.Context, log *slog.Logger, newUpdatesChan <-chan telego.Update) {
	timer := time.NewTimer(time.Second)
	skippedUpdates := 0

loop:
	for {
		select {
		case update := <-newUpdatesChan:
			timer.Reset(500 * time.Millisecond)
			skippedUpdates++
			log.DebugContext(ctx, "skipping queued update", "updateId", update.UpdateID)
		case <-ctx.Done():
			break loop
		case <-timer.C:
			break loop
		}
	}

	log.InfoContext(ctx, "skipped queued updates", "count", skippedUpdates)
}

type Option = func(*Bot)

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
