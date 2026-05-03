package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCloneStateAlreadyCloned(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "myrepo")
	if err := os.MkdirAll(filepath.Join(path, ".git"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	got, err := cloneState(path)
	if err != nil {
		t.Fatalf("cloneState: %v", err)
	}
	if got != stateAlreadyCloned {
		t.Fatalf("expected stateAlreadyCloned, got %v", got)
	}
}

func TestCloneStateMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nope")
	got, err := cloneState(path)
	if err != nil {
		t.Fatalf("cloneState: %v", err)
	}
	if got != stateMissing {
		t.Fatalf("expected stateMissing, got %v", got)
	}
}

func TestCloneStatePathExistsNotGit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plainfile")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	got, err := cloneState(path)
	if err != nil {
		t.Fatalf("cloneState: %v", err)
	}
	if got != statePathExistsNotGit {
		t.Fatalf("expected statePathExistsNotGit, got %v", got)
	}
	_ = errors.New
}
