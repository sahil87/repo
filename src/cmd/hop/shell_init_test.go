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
	if !strings.Contains(out, `command hop "$2" where`) {
		t.Fatalf("expected delegation to `command hop \"$2\" where` (repo-verb grammar), got:\n%s", out)
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

// TestShellInitContainsBareNameDispatch asserts the shim emits the bare-name
// branch (hop <name> with $# == 1 → _hop_dispatch cd "$1"). This is the
// 1-arg path of the new repo-first grammar: $1 is always treated as a repo
// name (the grammar is "subcommand xor repo" — never a tool).
func TestShellInitContainsBareNameDispatch(t *testing.T) {
	rootForCompletion = newRootCmd()
	defer func() { rootForCompletion = nil }()

	stdout, _, err := runArgs(t, "shell-init", "zsh")
	if err != nil {
		t.Fatalf("shell-init zsh: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, `_hop_dispatch cd "$1"`) {
		t.Fatalf("expected `_hop_dispatch cd \"$1\"` for 1-arg bare-name path, got:\n%s", out)
	}
}

// TestShellInitContainsCanonicalDashRRewrite asserts the shim rewrites the
// user-facing `hop <name> -R <cmd>...` form to the binary's internal
// `command hop -R <name> <cmd>...` shape. The shim flips; the binary's
// extractDashR keeps the existing argv shape (Design Decision #1).
func TestShellInitContainsCanonicalDashRRewrite(t *testing.T) {
	rootForCompletion = newRootCmd()
	defer func() { rootForCompletion = nil }()

	stdout, _, err := runArgs(t, "shell-init", "zsh")
	if err != nil {
		t.Fatalf("shell-init zsh: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, `command hop -R "$1" "${@:3}"`) {
		t.Fatalf("expected canonical -R rewrite `command hop -R \"$1\" \"${@:3}\"`, got:\n%s", out)
	}
}

// TestShellInitContainsToolFormDispatch asserts the shim emits the tool-form
// branch (hop <name> <tool> [args...] → command hop -R "$1" "$2" "${@:3}").
// This is the new sugar that runs a tool inside a repo with the repo name
// in $1 (repo-first grammar).
func TestShellInitContainsToolFormDispatch(t *testing.T) {
	rootForCompletion = newRootCmd()
	defer func() { rootForCompletion = nil }()

	stdout, _, err := runArgs(t, "shell-init", "zsh")
	if err != nil {
		t.Fatalf("shell-init zsh: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, `command hop -R "$1" "$2" "${@:3}"`) {
		t.Fatalf("expected tool-form dispatch `command hop -R \"$1\" \"$2\" \"${@:3}\"`, got:\n%s", out)
	}
}

// TestShellInitOmitsLegacyShape asserts the collapsed shim no longer emits
// the legacy precedence-ladder constructs: PATH inspection of $1, type-based
// builtin/keyword detection, or cheerful error printfs. After the flip, the
// grammar is "subcommand xor repo" in $1, so none of these checks apply.
func TestShellInitOmitsLegacyShape(t *testing.T) {
	rootForCompletion = newRootCmd()
	defer func() { rootForCompletion = nil }()

	stdout, _, err := runArgs(t, "shell-init", "zsh")
	if err != nil {
		t.Fatalf("shell-init zsh: %v", err)
	}
	out := stdout.String()

	if strings.Contains(out, `command -v "$1"`) {
		t.Errorf("expected NO `command -v \"$1\"` PATH check (removed in repo-first flip), got:\n%s", out)
	}
	if strings.Contains(out, `type "$1"`) {
		t.Errorf("expected NO `type \"$1\"` builtin detection (removed in repo-first flip), got:\n%s", out)
	}
	if strings.Contains(out, "is a shell builtin") {
		t.Errorf("expected NO `is a shell builtin` cheerful-error string (removed), got:\n%s", out)
	}
	if strings.Contains(out, "is not a known subcommand or a binary on PATH") {
		t.Errorf("expected NO `is not a known subcommand or a binary on PATH` cheerful-error (removed), got:\n%s", out)
	}
}

// TestShellInitZshDoesNotListCdOrWhereAsSubcommand asserts the case-list no
// longer treats `cd` or `where` as known subcommands at $1. Both verbs moved
// to $2 in the repo-verb grammar (`hop <name> cd`, `hop <name> where`); the
// top-level subcommands were removed.
//
// Same two-phase structure as TestShellInitZshDoesNotListCodeAsSubcommand:
// phase 1 anchors the case-list line, phase 2 asserts both tokens are absent.
func TestShellInitZshDoesNotListCdOrWhereAsSubcommand(t *testing.T) {
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

	if strings.Contains(caseListLine, "cd|") || strings.Contains(caseListLine, "|cd|") {
		t.Fatalf("expected `cd` to be removed from subcommand case-list (moved to $2 verb), got line:\n%s", caseListLine)
	}
	if strings.Contains(caseListLine, "where|") || strings.Contains(caseListLine, "|where|") {
		t.Fatalf("expected `where` to be removed from subcommand case-list (moved to $2 verb), got line:\n%s", caseListLine)
	}
}

// TestShellInitZshEmitsCdVerbBranch asserts the shim's repo-name branch
// includes the explicit-`cd`-verb dispatch routing through `_hop_dispatch cd "$1"`.
// The verb branches share a `$2 == "cd" || $2 == "where"` guard with extra-args
// forwarding to the binary; the inner cd arm calls `_hop_dispatch cd "$1"`.
func TestShellInitZshEmitsCdVerbBranch(t *testing.T) {
	rootForCompletion = newRootCmd()
	defer func() { rootForCompletion = nil }()

	stdout, _, err := runArgs(t, "shell-init", "zsh")
	if err != nil {
		t.Fatalf("shell-init zsh: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, `"$2" == "cd" || "$2" == "where"`) {
		t.Fatalf("expected combined verb guard `\"$2\" == \"cd\" || \"$2\" == \"where\"`, got:\n%s", out)
	}
	if !strings.Contains(out, `_hop_dispatch cd "$1"`) {
		t.Fatalf("expected `_hop_dispatch cd \"$1\"` (cd verb dispatch), got:\n%s", out)
	}
}

// TestShellInitZshEmitsWhereVerbBranch asserts the shim's repo-name branch
// routes the explicit `where` verb to `command hop "$1" where` (the binary's
// $2-verb dispatch).
func TestShellInitZshEmitsWhereVerbBranch(t *testing.T) {
	rootForCompletion = newRootCmd()
	defer func() { rootForCompletion = nil }()

	stdout, _, err := runArgs(t, "shell-init", "zsh")
	if err != nil {
		t.Fatalf("shell-init zsh: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, `command hop "$1" where`) {
		t.Fatalf("expected `command hop \"$1\" where` (where-verb dispatch to binary), got:\n%s", out)
	}
}

// TestShellInitZshVerbBranchForwardsExtraArgs asserts that when a verb at $2
// (cd or where) is followed by extra args, the shim forwards the full argv to
// the binary (`command hop "$@"`) so cobra's MaximumNArgs(2) raises an error
// rather than silently dropping the extra args.
func TestShellInitZshVerbBranchForwardsExtraArgs(t *testing.T) {
	rootForCompletion = newRootCmd()
	defer func() { rootForCompletion = nil }()

	stdout, _, err := runArgs(t, "shell-init", "zsh")
	if err != nil {
		t.Fatalf("shell-init zsh: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, `if [[ $# -gt 2 ]]; then`) {
		t.Fatalf("expected `if [[ $# -gt 2 ]]; then` (extra-args guard for verbs), got:\n%s", out)
	}
	// The body of the extra-args guard forwards to the binary verbatim.
	if !strings.Contains(out, `command hop "$@"`) {
		t.Fatalf("expected `command hop \"$@\"` forward when verb has extra args, got:\n%s", out)
	}
}

// TestShellInitZshDispatchCdHelperHasNoNoArg2Fallback asserts the shim's
// _hop_dispatch cd) arm does NOT contain the legacy no-$2 fallback
// (`if [[ -z "$2" ]]; then command hop cd`). The fallback was unreachable
// after the case-list dropped `cd` from $1 — the only callers (1-arg bare-name
// and 2-arg explicit-cd) always pass $1 as the dispatch's $2.
func TestShellInitZshDispatchCdHelperHasNoNoArg2Fallback(t *testing.T) {
	rootForCompletion = newRootCmd()
	defer func() { rootForCompletion = nil }()

	stdout, _, err := runArgs(t, "shell-init", "zsh")
	if err != nil {
		t.Fatalf("shell-init zsh: %v", err)
	}
	out := stdout.String()
	if strings.Contains(out, `command hop cd`) {
		t.Fatalf("expected NO `command hop cd` invocation (legacy no-$2 fallback removed), got:\n%s", out)
	}
}

// TestShellInitZshDoesNotListCodeAsSubcommand asserts the case-statement no
// longer treats `code` as a known subcommand (the binary's `hop code` was
// removed in favor of the tool-form `hop <repo> code`).
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

// TestShellInitZshDoesNotListOpenAsSubcommand asserts the case-list does not
// contain `open` after the repo-first grammar flip. The `hop open` subcommand
// was removed; users invoke `open` as a tool via the shim's tool-form sugar:
// `hop <name> open` (Darwin) or `hop <name> xdg-open` (Linux).
//
// Same two-phase structure as TestShellInitZshDoesNotListCodeAsSubcommand:
// phase 1 anchors the case-list line, phase 2 asserts `open` is absent.
func TestShellInitZshDoesNotListOpenAsSubcommand(t *testing.T) {
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

	if strings.Contains(caseListLine, "|open|") || strings.HasPrefix(strings.TrimSpace(caseListLine), "open|") {
		t.Fatalf("expected `open` to be removed from subcommand case-list (hop open subcommand removed), got line:\n%s", caseListLine)
	}
}

// TestShellInitZshListsHelpAsSubcommand asserts the case-list includes `help`
// so that `hop help` and `hop help <subcommand>` reach cobra's auto-generated
// help command. Without this, the shim would treat `hop help` as a bare-name
// `cd` (1 arg) or hit the tool-form path (`hop help where`, 2 args). Same
// two-phase structure as TestShellInitZshDoesNotListCodeAsSubcommand.
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
