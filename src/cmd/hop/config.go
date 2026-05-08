package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sahil87/hop/internal/config"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "config helpers (init, where, scan, print)",
	}
	cmd.AddCommand(newConfigInitCmd(), newConfigWhereCmd(), newConfigScanCmd(), newConfigPrintCmd())
	return cmd
}

func newConfigInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "bootstrap a starter hop.yaml at the resolved write target",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := config.ResolveWriteTarget()
			if err != nil {
				return err
			}
			if err := config.WriteStarter(target); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", target)
			fmt.Fprintln(cmd.ErrOrStderr(), "Edit the file to add your repos, or run `hop config scan <dir>` to populate from existing on-disk repos.")
			fmt.Fprintln(cmd.ErrOrStderr(), "Tip: set $HOP_CONFIG in your shell rc to point at a version-tracked location (a git-tracked dotfile, Dropbox, etc.) so this config moves with you across machines.")
			return nil
		},
	}
}

func newConfigWhereCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "where",
		Short: "print the resolved hop.yaml path (regardless of file existence)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := config.ResolveWriteTarget()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), target)
			return nil
		},
	}
}

// newConfigPrintCmd returns the cobra factory for `hop config print`. The
// subcommand resolves the active hop.yaml via config.Resolve() — the same
// reader-contract resolver used by every other read path — then streams the
// file's bytes verbatim to stdout. Comments and formatting are preserved by
// construction; no parsing happens here.
func newConfigPrintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "print",
		Short: "print the resolved hop.yaml contents to stdout",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.Resolve()
			if err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("hop config print: read %s: %w", path, err)
			}
			_, err = cmd.OutOrStdout().Write(data)
			return err
		},
	}
}

// scanLong is the cobra Long help for `hop config scan <dir>`. Pinned in
// T011 per spec § "External tool requirements" / intake § "Help text".
const scanLong = `Scan a directory for git repos and populate hop.yaml.

Auto-derives groups from the on-disk layout: repos at <code_root>/<org>/<name>
land in the 'default' flat group; non-convention repos land in invented
map-shaped groups keyed off the parent dir basename.

Examples:
  hop config scan ~/code              print the rendered YAML to stdout
  hop config scan ~/code --write      merge into the resolved hop.yaml in place
  hop config scan ~/code --depth 4    extend the walk one level deeper`

// newConfigScanCmd returns the cobra factory for `hop config scan <dir>`.
func newConfigScanCmd() *cobra.Command {
	var (
		write bool
		depth int
	)
	cmd := &cobra.Command{
		Use:   "scan <dir>",
		Short: "scan a directory for git repos and populate hop.yaml",
		Long:  scanLong,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigScan(cmd, args[0], depth, write)
		},
	}
	cmd.Flags().BoolVar(&write, "write", false, "merge results into the resolved hop.yaml (atomic, comment-preserving). Default: render to stdout.")
	cmd.Flags().IntVar(&depth, "depth", 3, "maximum DFS depth (root counts as depth 0; must be >= 1)")
	return cmd
}
