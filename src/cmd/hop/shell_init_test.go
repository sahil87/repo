package main

import (
	"errors"
	"strings"
	"testing"
)

func TestShellInitZshContainsHopFunctionAndAliases(t *testing.T) {
	// Set rootForCompletion so the completion script is appended (mirrors main()).
	rootForCompletion = newRootCmd()
	defer func() { rootForCompletion = nil }()

	stdout, _, err := runArgs(t, "shell-init", "zsh")
	if err != nil {
		t.Fatalf("shell-init zsh: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "hop()") {
		t.Fatalf("expected `hop()` function in output, got:\n%s", out)
	}
	if !strings.Contains(out, "_hop_dispatch") {
		t.Fatalf("expected `_hop_dispatch` helper, got:\n%s", out)
	}
	if !strings.Contains(out, `h() { hop "$@"; }`) {
		t.Fatalf("expected `h()` alias, got:\n%s", out)
	}
	if !strings.Contains(out, `hi() { command hop "$@"; }`) {
		t.Fatalf("expected `hi()` alias, got:\n%s", out)
	}
	if !strings.Contains(out, `command hop where "$2"`) {
		t.Fatalf("expected delegation to `command hop where`, got:\n%s", out)
	}
	// cobra-generated completion appends a `_hop` zsh completion function.
	if !strings.Contains(out, "_hop") {
		t.Fatalf("expected cobra-generated _hop completion, got:\n%s", out)
	}
}

// TestShellInitZshRegistersCompletionForAliases asserts that the emitted
// shell-init shares the cobra-generated _hop completion with the `h` and
// `hi` aliases via `compdef _hop h hi`. Without this, tab completion only
// works on the `hop` command — `h <prefix><TAB>` would fall through to
// zsh's default file-name completion.
func TestShellInitZshRegistersCompletionForAliases(t *testing.T) {
	rootForCompletion = newRootCmd()
	defer func() { rootForCompletion = nil }()

	stdout, _, err := runArgs(t, "shell-init", "zsh")
	if err != nil {
		t.Fatalf("shell-init zsh: %v", err)
	}
	if !strings.Contains(stdout.String(), "compdef _hop h hi") {
		t.Fatalf("expected `compdef _hop h hi` registration, got:\n%s", stdout.String())
	}
}

// TestShellInitZshRoutesCobraCompletionToBinary asserts that the emitted hop()
// shell function explicitly routes cobra's __complete* introspection calls
// to `command hop` rather than the bare-name dispatcher. Without this branch,
// the cobra-generated _hop completion script invokes the shell function with
// `__complete <args>...`, which falls through to the default case and is
// treated as a repo name (e.g. `cd __complete`) — breaking tab completion for
// any prefix beyond the empty case.
func TestShellInitZshRoutesCobraCompletionToBinary(t *testing.T) {
	rootForCompletion = newRootCmd()
	defer func() { rootForCompletion = nil }()

	stdout, _, err := runArgs(t, "shell-init", "zsh")
	if err != nil {
		t.Fatalf("shell-init zsh: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "__complete*)") {
		t.Fatalf("expected `__complete*)` case to forward cobra completion to `command hop`, got:\n%s", out)
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
	if withCode.msg != "hop shell-init: missing shell. Supported: zsh" {
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
	if withCode.msg != "hop shell-init: unsupported shell 'bash'. Supported: zsh" {
		t.Fatalf("unexpected message: %q", withCode.msg)
	}
}
