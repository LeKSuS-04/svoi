package logging

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"log/slog"
	"math/rand/v2"
)

type contextKey struct{}

type contextHandler struct {
	h slog.Handler
}

func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	attrs := ctx.Value(contextKey{})
	if attrs != nil {
		r.AddAttrs(attrs.([]slog.Attr)...)
	}
	return h.h.Handle(ctx, r)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{h: h.h.WithAttrs(attrs)}
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{h: h.h.WithGroup(name)}
}

func (h *contextHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.h.Enabled(ctx, l)
}

func PopulateContext(ctx context.Context, attrs ...slog.Attr) context.Context {
	oldAttrs := ctx.Value(contextKey{})
	if oldAttrs != nil {
		attrs = append(oldAttrs.([]slog.Attr), attrs...)
	}
	return context.WithValue(ctx, contextKey{}, attrs)
}

func PopulateContextID(ctx context.Context, key string) context.Context {
	return PopulateContext(ctx, slog.String(key, makeContextID()))
}

func makeContextID() string {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], rand.Uint64())
	return hex.EncodeToString(b[:])
}
