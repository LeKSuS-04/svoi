package logging

import (
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/phsym/console-slog"
	"golang.org/x/term"
)

func New(name string) *slog.Logger {
	return slog.Default().With(slog.String("logger", name))
}

func init() {
	Configure()
}

func Configure(opts ...logConfigOption) {
	cfg := discoverDefaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	var handler slog.Handler
	switch cfg.handlerType {
	case HandlerTypeTerminal:
		handler = console.NewHandler(cfg.output, &console.HandlerOptions{
			Level:      cfg.level,
			TimeFormat: "2006-01-02 15:04:05.000",
		})

	case HandlerTypeBasic:
		handler = slog.NewTextHandler(cfg.output, &slog.HandlerOptions{
			Level: cfg.level,
		})

	case HandlerTypeJSON:
		handler = slog.NewJSONHandler(cfg.output, &slog.HandlerOptions{
			Level: cfg.level,
		})

	default:
		panic("unknown handler type")
	}

	slog.SetDefault(slog.New(&contextHandler{handler}))
}

func discoverDefaultConfig() *logConfig {
	cfg := &logConfig{
		level:       slog.LevelInfo,
		handlerType: HandlerTypeBasic,
		output:      os.Stdout,
	}

	if os.Getenv("DEBUG") != "" || testing.Testing() {
		cfg.level = slog.LevelDebug
	}

	if testing.Testing() {
		cfg.handlerType = HandlerTypeBasic
	} else if term.IsTerminal(int(os.Stdout.Fd())) {
		cfg.handlerType = HandlerTypeTerminal
	} else {
		cfg.handlerType = HandlerTypeJSON
	}

	return cfg
}

type HandlerType string

const (
	HandlerTypeBasic    HandlerType = "basic"
	HandlerTypeTerminal HandlerType = "terminal"
	HandlerTypeJSON     HandlerType = "json"
)

type logConfig struct {
	level       slog.Level
	handlerType HandlerType
	output      io.Writer
}

type logConfigOption func(*logConfig)

func WithHandlerType(handlerType HandlerType) logConfigOption {
	return func(cfg *logConfig) {
		cfg.handlerType = handlerType
	}
}

func WithLevel(level slog.Level) logConfigOption {
	return func(cfg *logConfig) {
		cfg.level = level
	}
}

func WithOutput(output io.Writer) logConfigOption {
	return func(cfg *logConfig) {
		cfg.output = output
	}
}
