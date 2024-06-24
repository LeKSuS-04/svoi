package bot

import (
	"context"
	"fmt"
	"sync"

	"github.com/LeKSuS-04/svoi-bot/internal/db"
	"github.com/mymmrac/telego"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
)

type worker struct {
	api          *telego.Bot
	messageCache *cache.Cache
	db           *db.DB
	log          *logrus.Entry
	updates      <-chan telego.Update
}

func (b *Bot) Run(ctx context.Context) error {
	b.log.Debug("Running in debug mode")

	db, err := db.New(b.dbPath)
	if err != nil {
		return fmt.Errorf("create db: %w", err)
	}

	messageCache := cache.New(b.cacheDuration, b.cacheCleanupInterval)
	workerUpdatesChan := make(chan telego.Update, 1000)

	wg := sync.WaitGroup{}
	wg.Add(b.workerCount)
	for i := range b.workerCount {
		workerId := i
		go func() {
			defer wg.Done()
			w := worker{
				api:          b.api,
				messageCache: messageCache,
				db:           db,
				log:          b.log.WithField("worker", workerId),
				updates:      workerUpdatesChan,
			}
			w.Work(ctx)
		}()
	}

	newUpdatesChan, err := b.api.UpdatesViaLongPolling(nil)
	if err != nil {
		return fmt.Errorf("subscribe to updates: %w", err)
	}

	self, err := b.api.GetMe()
	if err != nil {
		return fmt.Errorf("get self: %w", err)
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
