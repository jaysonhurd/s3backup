package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/rs/zerolog"
)

// TestFromLegacy verifies filtering and bound, grouped, and resolved attributes.
func TestFromLegacy(t *testing.T) {
	var output bytes.Buffer
	legacy := zerolog.New(&output).Level(zerolog.InfoLevel)
	logger := FromLegacy(&legacy).With("service", "backup").WithGroup("request").With("bucket", "test")
	logger.Debug("hidden")
	logger.Info("upload", slog.Int("size", 5), slog.Group("nested", slog.String("key", "file")))
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	request := record["request"].(map[string]any)
	if record["message"] != "upload" || record["service"] != "backup" || request["bucket"] != "test" || request["size"] != float64(5) || request["nested"].(map[string]any)["key"] != "file" {
		t.Fatalf("record = %#v", record)
	}
	FromLegacy(nil).Error("discarded")
}

// TestFatalSeverity preserves fatal-only filtering without exiting the host.
func TestFatalSeverity(t *testing.T) {
	var output bytes.Buffer
	legacy := zerolog.New(&output).Level(zerolog.FatalLevel)
	logger := FromLegacy(&legacy)
	logger.Error("filtered")
	logger.Log(context.Background(), LevelFatal, "failure")
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record["level"] != "fatal" || record["message"] != "failure" {
		t.Fatalf("record = %v", record)
	}
}
