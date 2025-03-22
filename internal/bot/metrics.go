package bot

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"

	"github.com/LeKSuS-04/svoi-bot/internal/db"
)

const (
	labelChatID       = "chat_id"
	labelCommand      = "command"
	labelUpdateType   = "update_type"
	labelTriggerType  = "trigger_type"
	labelResponseType = "response_type"
)

var (
	successfulUpdates = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "successful_updates_count",
			Help: "Number of updates processed successfully",
		},
		[]string{labelChatID, labelUpdateType},
	)

	failedUpdates = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "failed_updates_count",
			Help: "Number of updates processed with error",
		},
		[]string{labelChatID, labelUpdateType},
	)

	updateDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "update_duration_seconds",
			Help:    "Duration of update processing",
			Buckets: []float64{0.001, 0.01, 0.1, 0.25, 0.5, 0.75, 1, 2, 5, 10},
		},
		[]string{labelChatID, labelUpdateType},
	)

	commandUsageStatistics = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "command_usage_count",
			Help: "Number of times a command was used",
		},
		[]string{labelChatID, labelCommand},
	)

	triggerTypeStatistics = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "trigger_type_count",
			Help: "Number of times a trigger type was used",
		},
		[]string{labelChatID, labelTriggerType},
	)

	responseTypeStatistics = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "response_type_count",
			Help: "Number of times a response type was used",
		},
		[]string{labelChatID, labelResponseType},
	)

	totalUsers = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "total_users_count",
			Help: "Total number of users",
		},
	)

	totalChats = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "total_chats_count",
			Help: "Total number of chats",
		},
	)
)

func (b *Bot) runMetricsServer(ctx context.Context) {
	logger := b.log.WithField("component", "metrics")
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		for {
			select {
			case <-ctx.Done():
				return

			default:
				logger.Debug("Starting metrics server")
				if err := http.ListenAndServe(b.config.Metrics.Addr, nil); err != nil {
					logger.WithError(err).Error("Failed to start metrics server")
				}
				time.Sleep(1 * time.Second)
			}
		}
	}()

	go func() {
		connector := &db.SimpleConnector{DbPath: b.dbPath}
		var db *db.DB
		var err error

		ticker := time.NewTicker(b.config.Metrics.UpdatePeriod)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case <-ticker.C:
			}

			if db == nil {
				db, err = connector.Connect()
				if err != nil {
					db = nil
					logger.WithError(err).Error("Failed to connect to database")
					continue
				}
			}

			stats, err := db.GetStats(ctx)
			if err != nil {
				db = nil
				logger.WithError(err).Error("Failed to get stats")
				continue
			}
			logger.WithFields(logrus.Fields{
				"total_users": stats.TotalUsers,
				"total_chats": stats.TotalChats,
			}).Debug("Stats")
			totalUsers.Set(float64(stats.TotalUsers))
			totalChats.Set(float64(stats.TotalChats))
		}
	}()
}
