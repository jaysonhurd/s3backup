// Package logging adapts legacy logger dependencies to structured slog records.
package logging

import (
	"context"
	"io"
	"log/slog"

	"github.com/rs/zerolog"
)

// LevelFatal preserves fatal severity without taking ownership of process exits.
const LevelFatal = slog.LevelError + 4

// FromLegacy retains the caller's output and filtering while using slog internally.
func FromLegacy(logger *zerolog.Logger) *slog.Logger {
	if logger == nil {
		return slog.New(slog.NewJSONHandler(io.Discard, nil))
	}
	return slog.New(&handler{logger: logger})
}

type handler struct {
	logger *zerolog.Logger
	attrs  []slog.Attr
	groups []string
}

// Enabled honors the legacy logger's level and the global level.
func (h *handler) Enabled(_ context.Context, level slog.Level) bool {
	return legacyLevel(level) >= h.logger.GetLevel() && legacyLevel(level) >= zerolog.GlobalLevel()
}

// Handle forwards resolved attributes without formatting them into the message.
func (h *handler) Handle(_ context.Context, record slog.Record) error {
	fields := map[string]any{}
	add := func(attr slog.Attr) { addAttr(fields, attr) }
	for _, attr := range h.attrs {
		add(attr)
	}
	current := fields
	for _, group := range h.groups {
		nested, ok := current[group].(map[string]any)
		if !ok {
			nested = map[string]any{}
			current[group] = nested
		}
		current = nested
	}
	record.Attrs(func(attr slog.Attr) bool { addAttr(current, attr); return true })
	h.logger.WithLevel(legacyLevel(record.Level)).Fields(fields).Msg(record.Message)
	return nil
}

// WithAttrs binds attributes at the current group depth.
func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.attrs = append([]slog.Attr{}, h.attrs...)
	for _, attr := range attrs {
		for i := len(h.groups) - 1; i >= 0; i-- {
			attr = slog.Group(h.groups[i], attr)
		}
		clone.attrs = append(clone.attrs, attr)
	}
	return &clone
}

// WithGroup scopes subsequent attributes.
func (h *handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := *h
	clone.groups = append(append([]string{}, h.groups...), name)
	return &clone
}

// legacyLevel maps standard slog levels to the corresponding zerolog levels.
func legacyLevel(level slog.Level) zerolog.Level {
	switch {
	case level >= LevelFatal:
		return zerolog.FatalLevel
	case level >= slog.LevelError:
		return zerolog.ErrorLevel
	case level >= slog.LevelWarn:
		return zerolog.WarnLevel
	case level >= slog.LevelInfo:
		return zerolog.InfoLevel
	default:
		return zerolog.DebugLevel
	}
}

// addAttr resolves values and preserves group structure.
func addAttr(fields map[string]any, attr slog.Attr) {
	attr.Value = attr.Value.Resolve()
	if attr.Equal(slog.Attr{}) {
		return
	}
	if attr.Value.Kind() == slog.KindGroup {
		target := fields
		if attr.Key != "" {
			nested, ok := fields[attr.Key].(map[string]any)
			if !ok {
				nested = map[string]any{}
				fields[attr.Key] = nested
			}
			target = nested
		}
		for _, child := range attr.Value.Group() {
			addAttr(target, child)
		}
		return
	}
	fields[attr.Key] = attr.Value.Any()
}
