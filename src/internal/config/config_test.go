package config

import (
	"bytes"
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
	if cfg.CodeRoot != "~/code" {
		t.Errorf("CodeRoot = %q, want ~/code", cfg.CodeRoot)
	}
	if len(cfg.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(cfg.Groups))
	}
	if cfg.Groups[0].Name != "default" {
		t.Errorf("Groups[0].Name = %q, want default", cfg.Groups[0].Name)
	}
	if len(cfg.Groups[0].URLs) != 2 {
		t.Errorf("expected 2 default URLs, got %d", len(cfg.Groups[0].URLs))
	}
	if cfg.Groups[1].Name != "external" {
		t.Errorf("Groups[1].Name = %q, want external", cfg.Groups[1].Name)
	}
}

func TestLoadEmpty(t *testing.T) {
	cfg, err := Load("testdata/empty.yaml")
	if err != nil {
		t.Fatalf("Load empty.yaml: %v", err)
	}
	if cfg.CodeRoot != "~" {
		t.Errorf("CodeRoot = %q, want ~", cfg.CodeRoot)
	}
	if len(cfg.Groups) != 0 {
		t.Fatalf("expected 0 groups, got %d", len(cfg.Groups))
	}
}

func TestLoadMixed(t *testing.T) {
	cfg, err := Load("testdata/valid-mixed.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(cfg.Groups))
	}
	want := []struct {
		name string
		dir  string
	}{
		{"default", ""},
		{"vendor", "~/vendor"},
		{"experiments", "experiments"},
	}
	for i, w := range want {
		if cfg.Groups[i].Name != w.name {
			t.Errorf("Groups[%d].Name = %q, want %q", i, cfg.Groups[i].Name, w.name)
		}
		if cfg.Groups[i].Dir != w.dir {
			t.Errorf("Groups[%d].Dir = %q, want %q", i, cfg.Groups[i].Dir, w.dir)
		}
	}
}

func TestLoadEmptyGroup(t *testing.T) {
	cfg, err := Load("testdata/valid-empty-group.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(cfg.Groups))
	}
	if len(cfg.Groups[0].URLs) != 0 {
		t.Errorf("default URLs = %v, want empty", cfg.Groups[0].URLs)
	}
	if cfg.Groups[1].Dir != "~/code/experiments" {
		t.Errorf("experiments.Dir = %q", cfg.Groups[1].Dir)
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

func TestLoadInvalidGroupName(t *testing.T) {
	_, err := Load("testdata/invalid-group-name.yaml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid group name") {
		t.Errorf("error: %v", err)
	}
}

func TestLoadInvalidUnknownTop(t *testing.T) {
	_, err := Load("testdata/invalid-unknown-top.yaml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown top-level field 'servers'") {
		t.Errorf("error: %v", err)
	}
}

func TestLoadInvalidURLCollision(t *testing.T) {
	_, err := Load("testdata/invalid-url-collision.yaml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "appears in groups") {
		t.Errorf("error: %v", err)
	}
}

func TestLoadInvalidEmptyDir(t *testing.T) {
	_, err := Load("testdata/invalid-empty-dir.yaml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "empty 'dir'") {
		t.Errorf("error: %v", err)
	}
}

func TestLoadInvalidUnknownGroupKey(t *testing.T) {
	_, err := Load("testdata/invalid-unknown-group-key.yaml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown field 'sync_strategy'") {
		t.Errorf("error: %v", err)
	}
}

func TestLoadMissingRepos(t *testing.T) {
	_, err := Load("testdata/missing-repos.yaml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "missing required field 'repos'") {
		t.Errorf("error: %v", err)
	}
}

func TestLoadDupInGroup(t *testing.T) {
	_, err := Load("testdata/dup-in-group.yaml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "is listed twice in group") {
		t.Errorf("error: %v", err)
	}
}

func TestWriteStarterCreatesFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "sub", "hop.yaml")
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
	target := filepath.Join(dir, "hop.yaml")
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
	if !strings.Contains(err.Error(), "$HOP_CONFIG") {
		t.Errorf("expected $HOP_CONFIG in error, got %q", err.Error())
	}

	got, _ := os.ReadFile(target)
	if !bytes.Equal(got, original) {
		t.Fatalf("file content modified after refusal")
	}
}

// TestStarterParses ensures the embedded starter content parses successfully
// against the new schema — it would catch drift between starter.yaml and the
// loader's expectations.
func TestStarterParses(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "hop.yaml")
	if err := WriteStarter(target); err != nil {
		t.Fatalf("WriteStarter: %v", err)
	}
	if _, err := Load(target); err != nil {
		t.Fatalf("starter does not parse: %v", err)
	}
}
