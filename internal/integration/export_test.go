package integration

import (
	"bytes"
	"testing"
	"time"
)

func TestExportRoundTrip(t *testing.T) {
	t.Parallel()

	exported, err := NewExport("integration", Recipe{
		Base:     "main",
		Branches: []string{"feature-a", "feature-b"},
	})
	if err != nil {
		t.Fatalf("NewExport() error = %v", err)
	}

	var buf bytes.Buffer
	if err := EncodeExport(&buf, exported); err != nil {
		t.Fatalf("EncodeExport() error = %v", err)
	}

	decoded, err := DecodeExport(&buf)
	if err != nil {
		t.Fatalf("DecodeExport() error = %v", err)
	}

	if decoded.Integration.Name != "integration" {
		t.Fatalf("Name = %q, want integration", decoded.Integration.Name)
	}
	if decoded.Integration.Base != "main" {
		t.Fatalf("Base = %q, want main", decoded.Integration.Base)
	}
	if got := len(decoded.Integration.Branches); got != 2 {
		t.Fatalf("len(Branches) = %d, want 2", got)
	}
}

func TestDecodeExport(t *testing.T) {
	t.Parallel()

	state, err := DecodeExport(bytes.NewBufferString(`{"version":1,"exported_at":"2026-04-07T14:30:00Z","integration":{"name":"integration","base":"main","branches":["feature-a","feature-b"]}}`))
	if err != nil {
		t.Fatalf("DecodeExport() error = %v", err)
	}
	if state.Version != 1 {
		t.Fatalf("Version = %d, want 1", state.Version)
	}
	if !state.ExportedAt.Equal(time.Date(2026, 4, 7, 14, 30, 0, 0, time.UTC)) {
		t.Fatalf("ExportedAt = %v, want fixed time", state.ExportedAt)
	}
}
