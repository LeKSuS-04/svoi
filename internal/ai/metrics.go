package ai

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	successfulGenerations = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "successful_generations_count",
			Help: "Number of successful generations",
		},
	)

	failedGenerations = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "failed_generations_count",
			Help: "Number of failed generations",
		},
	)

	generationDurationSeconds = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "generation_duration_seconds",
			Help:    "Duration of generation",
			Buckets: []float64{0.05, 0.1, 0.2, 0.5, 1, 2, 3, 4, 5, 10, 20, 30},
		},
	)
)
