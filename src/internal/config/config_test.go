package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadValid(t *testing.T) {
	cfg, err := Load("testdata/valid.yaml")
	if err != nil {
		t.Fatalf("Load valid.yaml: %v", err)
	}
	if len(cfg.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(cfg.Entries))
	}
	if urls := cfg.Entries["~/code/sahil87"]; len(urls) != 2 {
		t.Fatalf("expected 2 sahil87 URLs, got %d", len(urls))
	}
	if urls := cfg.Entries["~/code/wvrdz"]; len(urls) != 1 {
		t.Fatalf("expected 1 wvrdz URL, got %d", len(urls))
	}
}

func TestLoadEmpty(t *testing.T) {
	cfg, err := Load("testdata/empty.yaml")
	if err != nil {
		t.Fatalf("Load empty.yaml: %v", err)
	}
	if len(cfg.Entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(cfg.Entries))
	}
}

func TestLoadMalformed(t *testing.T) {
	_, err := Load("testdata/malformed.yaml")
	if err == nil {
		t.Fatalf("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), "testdata/malformed.yaml") {
		t.Fatalf("expected file path in error, got %q", err.Error())
	}
}

func TestLoadNonexistent(t *testing.T) {
	_, err := Load("/nonexistent/no-such-file.yaml")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "/nonexistent/no-such-file.yaml") {
		t.Fatalf("expected file path in error, got %q", err.Error())
	}
}

func TestWriteStarterCreatesFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "sub", "repos.yaml")
	if err := WriteStarter(target); err != nil {
		t.Fatalf("WriteStarter: %v", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("expected mode 0644, got %o", info.Mode().Perm())
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, StarterContent()) {
		t.Fatalf("starter content mismatch")
	}
}

func TestWriteStarterRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "repos.yaml")
	original := []byte("preexisting\n")
	if err := os.WriteFile(target, original, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	err := WriteStarter(target)
	if err == nil {
		t.Fatalf("expected refusal, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected 'already exists' message, got %q", err.Error())
	}

	got, _ := os.ReadFile(target)
	if !bytes.Equal(got, original) {
		t.Fatalf("file content modified after refusal")
	}

	// Suppress unused import warning if any
	_ = errors.Is
}
