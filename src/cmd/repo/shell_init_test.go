package main

import (
	"errors"
	"strings"
	"testing"
)

func TestShellInitZshContainsRepoFunctionAndCompdef(t *testing.T) {
	stdout, _, err := runArgs(t, "shell-init", "zsh")
	if err != nil {
		t.Fatalf("shell-init zsh: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "repo()") {
		t.Fatalf("expected `repo()` function in output, got:\n%s", out)
	}
	if !strings.Contains(out, "compdef _repo repo") {
		t.Fatalf("expected `compdef _repo repo` line, got:\n%s", out)
	}
	if !strings.Contains(out, `command repo path "$2"`) {
		t.Fatalf("expected delegation to `command repo path`, got:\n%s", out)
	}
}

func TestShellInitMissingShell(t *testing.T) {
	_, _, err := runArgs(t, "shell-init")
	if err == nil {
		t.Fatalf("expected error when no shell arg")
	}
	var withCode *errExitCode
	if !errors.As(err, &withCode) {
		t.Fatalf("expected *errExitCode, got %T", err)
	}
	if withCode.code != 2 {
		t.Fatalf("expected exit 2, got %d", withCode.code)
	}
	if withCode.msg != "repo shell-init: missing shell. Supported: zsh" {
		t.Fatalf("unexpected message: %q", withCode.msg)
	}
}

func TestShellInitUnsupportedShell(t *testing.T) {
	_, _, err := runArgs(t, "shell-init", "bash")
	if err == nil {
		t.Fatalf("expected error for unsupported shell")
	}
	var withCode *errExitCode
	if !errors.As(err, &withCode) {
		t.Fatalf("expected *errExitCode, got %T", err)
	}
	if withCode.code != 2 {
		t.Fatalf("expected exit 2, got %d", withCode.code)
	}
	if withCode.msg != "repo shell-init: unsupported shell 'bash'. Supported: zsh" {
		t.Fatalf("unexpected message: %q", withCode.msg)
	}
}
