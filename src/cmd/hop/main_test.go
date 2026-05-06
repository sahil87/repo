package main

import "testing"

// TestIsCompletionInvocationRecognizesCompleteEntrypoints verifies the
// helper accepts both cobra completion entrypoints (__complete and
// __completeNoDesc) at args[1]. These are the two shapes produced by
// cobra's generated zsh/bash completion scripts — gating the pre-cobra
// extractDashR skip on either ensures `hop -R <TAB>` reaches the root's
// ValidArgsFunction.
func TestIsCompletionInvocationRecognizesCompleteEntrypoints(t *testing.T) {
	cases := [][]string{
		{"hop", "__complete", "-R", "", ""},
		{"hop", "__completeNoDesc", "where", ""},
	}
	for _, args := range cases {
		if !isCompletionInvocation(args) {
			t.Errorf("isCompletionInvocation(%v) = false, want true", args)
		}
	}
}

// TestIsCompletionInvocationRejectsNormalInvocations guards against the
// helper accidentally suppressing extractDashR for real `-R` calls or any
// other normal subcommand invocation.
func TestIsCompletionInvocationRejectsNormalInvocations(t *testing.T) {
	cases := [][]string{
		{"hop", "-R", "name", "ls"},
		{"hop", "ls"},
		{"hop", "where", "alpha"},
		{"hop"},
		{},
	}
	for _, args := range cases {
		if isCompletionInvocation(args) {
			t.Errorf("isCompletionInvocation(%v) = true, want false", args)
		}
	}
}
