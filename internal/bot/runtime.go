package bot

import (
	"context"
	"fmt"
	"sync"

	"github.com/mymmrac/telego"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"

	"github.com/LeKSuS-04/svoi-bot/internal/ai"
	"github.com/LeKSuS-04/svoi-bot/internal/db"
)

type worker struct {
	config  *Config
	api     *telego.Bot
	cache   *cache.Cache
	db      *db.DB
	ai      *ai.AI
	log     *logrus.Entry
	updates <-chan telego.Update
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

	wg := sync.WaitGroup{}
	wg.Add(b.workerCount)
	for i := range b.workerCount {
		workerId := i
		go func() {
			defer wg.Done()
			w := worker{
				config: b.config,
				api:    b.api,
				cache:  cache,
				db:     db,
				ai:     aiHandler,
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

	self, err := b.api.GetMe()
	if err != nil {
		return fmt.Errorf("get self: %w", err)
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

func (w *worker) Work(ctx context.Context) {
	w.log.Info("Launched worker")

loop:
	for {
		select {
		case <-ctx.Done():
			break loop

		case update := <-w.updates:
			err := w.handleUpdate(ctx, update)
			if err != nil {
				w.log.
					WithField("updateId", update.UpdateID).
					WithError(err).
					Error("Failed to handle update")
			} else {
				w.log.WithField("updateId", update.UpdateID).Debug("Successfully handled update")
			}
		}
	}

	w.log.Info("Stopped worker")
}
