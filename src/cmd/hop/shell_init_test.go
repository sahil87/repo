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

// TestShellInitContainsToolFormDispatch asserts the shim emits the tool-form
// branch (hop <tool> <repo> [args...] → command hop -R "$2" "$1" "${@:3}").
// This is the new sugar that replaces `hop code` and lets users invoke any
// PATH binary in any registered repo without typing `-R` explicitly.
func TestShellInitContainsToolFormDispatch(t *testing.T) {
	rootForCompletion = newRootCmd()
	defer func() { rootForCompletion = nil }()

	stdout, _, err := runArgs(t, "shell-init", "zsh")
	if err != nil {
		t.Fatalf("shell-init zsh: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, `command -v "$1"`) {
		t.Fatalf("expected `command -v \"$1\"` PATH check for tool-form, got:\n%s", out)
	}
	if !strings.Contains(out, `command hop -R "$2" "$1" "${@:3}"`) {
		t.Fatalf("expected tool-form dispatch `command hop -R \"$2\" \"$1\" \"${@:3}\"`, got:\n%s", out)
	}
}

// TestShellInitEmitsCheerfulBuiltinError asserts the shim emits a helpful
// stderr message when the user types `hop <builtin> <repo>` (e.g. `hop pwd
// dotfiles`). Without this, the call would fall through to the binary,
// which errors with cobra's terse "accepts at most 1 arg(s)" — useless for
// the user to debug. The cheerful error suggests `hop where <repo>` (path)
// and `hop -R <repo> /full/path/to/<tool>` (binary equivalent).
//
// The message uses `type "$1"` to detect the kind (builtin / keyword /
// alias / function) so an alias or function gets an accurate label rather
// than being mislabeled as a builtin.
func TestShellInitEmitsCheerfulBuiltinError(t *testing.T) {
	rootForCompletion = newRootCmd()
	defer func() { rootForCompletion = nil }()

	stdout, _, err := runArgs(t, "shell-init", "zsh")
	if err != nil {
		t.Fatalf("shell-init zsh: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, `_hop_kind="$(type "$1" 2>&1 | head -1)"`) {
		t.Fatalf("expected `type \"$1\"` kind detection in builtin error, got:\n%s", out)
	}
	if !strings.Contains(out, "not a binary, so it can't run as a tool inside a repo.") {
		t.Fatalf("expected cheerful not-a-binary message, got:\n%s", out)
	}
	if !strings.Contains(out, "To get the path: hop where") {
		t.Fatalf("expected `hop where` hint in builtin error, got:\n%s", out)
	}
	if !strings.Contains(out, "hop -R %s /full/path/to/%s") {
		t.Fatalf("expected `hop -R` binary-equivalent hint in builtin error, got:\n%s", out)
	}
}

// TestShellInitEmitsCheerfulMissingBinaryError asserts the shim emits a
// helpful stderr message when the user types `hop <typo> <repo>` where
// `<typo>` is not a known subcommand AND not a binary on PATH. Without
// this, the call would fall through to cobra's terse error.
func TestShellInitEmitsCheerfulMissingBinaryError(t *testing.T) {
	rootForCompletion = newRootCmd()
	defer func() { rootForCompletion = nil }()

	stdout, _, err := runArgs(t, "shell-init", "zsh")
	if err != nil {
		t.Fatalf("shell-init zsh: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "is not a known subcommand or a binary on PATH") {
		t.Fatalf("expected cheerful missing-binary message, got:\n%s", out)
	}
}

// TestShellInitZshDoesNotListCodeAsSubcommand asserts the case-statement no
// longer treats `code` as a known subcommand (the binary's `hop code` was
// removed in favor of the tool-form `hop code <repo>`).
//
// The test is structured in two phases so that a missing-or-renamed case-list
// line cannot make the test silently no-op (which the original loop-only
// version did): phase 1 finds the line; phase 2 asserts `code` is absent.
// If the case-list line is ever renamed, removed, or re-shaped, phase 1 fails.
func TestShellInitZshDoesNotListCodeAsSubcommand(t *testing.T) {
	rootForCompletion = newRootCmd()
	defer func() { rootForCompletion = nil }()

	stdout, _, err := runArgs(t, "shell-init", "zsh")
	if err != nil {
		t.Fatalf("shell-init zsh: %v", err)
	}
	out := stdout.String()

	// Phase 1: locate the subcommand case-list line. The shim emits the line
	// `<sub>|<sub>|...|completion)` followed on the next line by
	// `_hop_dispatch "$@"`. We anchor on the trailing `|completion)` form
	// which uniquely identifies the case-list line — robust to subcommand
	// list reordering and indentation changes.
	var caseListLine string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "|completion)") && strings.Contains(line, "shell-init") {
			caseListLine = line
			break
		}
	}
	if caseListLine == "" {
		t.Fatalf("could not find subcommand case-list line (anchor: `|completion)` + `shell-init` on one line). The shim format may have changed; update this test. Output:\n%s", out)
	}

	// Phase 2: assert `code` is absent from the located line.
	if strings.Contains(caseListLine, "|code|") || strings.HasPrefix(strings.TrimSpace(caseListLine), "code|") {
		t.Fatalf("expected `code` to be removed from subcommand case-list, got line:\n%s", caseListLine)
	}
}

// TestShellInitZshListsHelpAsSubcommand asserts the case-list includes `help`
// so that `hop help` and `hop help <subcommand>` reach cobra's auto-generated
// help command. Without this, the shim would treat `hop help` as a bare-name
// `cd` (1 arg) or hit the tool-form/cheerful-error path (`hop help open`,
// 2 args). Same two-phase structure as TestShellInitZshDoesNotListCodeAsSubcommand.
func TestShellInitZshListsHelpAsSubcommand(t *testing.T) {
	rootForCompletion = newRootCmd()
	defer func() { rootForCompletion = nil }()

	stdout, _, err := runArgs(t, "shell-init", "zsh")
	if err != nil {
		t.Fatalf("shell-init zsh: %v", err)
	}
	out := stdout.String()

	var caseListLine string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "|completion)") && strings.Contains(line, "shell-init") {
			caseListLine = line
			break
		}
	}
	if caseListLine == "" {
		t.Fatalf("could not find subcommand case-list line. Output:\n%s", out)
	}

	if !strings.Contains(caseListLine, "|help|") {
		t.Fatalf("expected `help` in the subcommand case-list (so `hop help` reaches cobra), got line:\n%s", caseListLine)
	}
}

func TestShellInitBashEmitsFunctionAndCompletion(t *testing.T) {
	rootForCompletion = newRootCmd()
	defer func() { rootForCompletion = nil }()

	stdout, _, err := runArgs(t, "shell-init", "bash")
	if err != nil {
		t.Fatalf("shell-init bash: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "hop()") {
		t.Fatalf("expected `hop()` function in output, got:\n%s", out)
	}
	if !strings.Contains(out, `h() { hop "$@"; }`) {
		t.Fatalf("expected `h()` alias, got:\n%s", out)
	}
	// Bash uses `complete -F __start_hop` (not `compdef`).
	if !strings.Contains(out, "complete -o default -F __start_hop h hi") {
		t.Fatalf("expected bash `complete -F __start_hop h hi`, got:\n%s", out)
	}
	if !strings.Contains(out, "__start_hop") {
		t.Fatalf("expected cobra-generated `__start_hop` completion fn, got:\n%s", out)
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
	if !strings.Contains(withCode.msg, "missing shell") {
		t.Fatalf("unexpected message: %q", withCode.msg)
	}
	if !strings.Contains(withCode.msg, "zsh") || !strings.Contains(withCode.msg, "bash") {
		t.Fatalf("expected message to mention both zsh and bash: %q", withCode.msg)
	}
}

func TestShellInitUnsupportedShell(t *testing.T) {
	_, _, err := runArgs(t, "shell-init", "fish")
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
	if !strings.Contains(withCode.msg, "unsupported shell 'fish'") {
		t.Fatalf("unexpected message: %q", withCode.msg)
	}
}
