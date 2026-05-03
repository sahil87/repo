package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

// runArgs constructs a fresh root command, captures stdout/stderr buffers, executes
// with the provided args, and returns the buffers and any error from cobra.
func runArgs(t *testing.T, args ...string) (stdout, stderr *bytes.Buffer, err error) {
	t.Helper()
	cmd := newRootCmd()
	stdout = &bytes.Buffer{}
	stderr = &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return stdout, stderr, err
}

// runCmd executes a single subcommand factory directly (without the root) and
// returns its captured buffers. Useful when you want to test a command in isolation.
func runCmd(t *testing.T, factory func() *cobra.Command, args ...string) (stdout, stderr *bytes.Buffer, err error) {
	t.Helper()
	cmd := factory()
	stdout = &bytes.Buffer{}
	stderr = &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return stdout, stderr, err
}

// writeReposFixture writes a repos.yaml under t.TempDir() and points $REPOS_YAML at it.
// Returns the full path. yamlBody is written verbatim.
func writeReposFixture(t *testing.T, yamlBody string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "repos.yaml")
	if err := os.WriteFile(path, []byte(yamlBody), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	t.Setenv("REPOS_YAML", path)
	t.Setenv("XDG_CONFIG_HOME", "")
	os.Unsetenv("XDG_CONFIG_HOME")
	return path
}

// clearConfigEnv unsets all three config env vars.
func clearConfigEnv(t *testing.T) {
	t.Helper()
	os.Unsetenv("REPOS_YAML")
	os.Unsetenv("XDG_CONFIG_HOME")
	t.Setenv("HOME", os.Getenv("HOME")) // preserve HOME for ~ expansion in tests that rely on it
}
