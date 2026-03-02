package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/louloulibs/pipelink/internal/config"
	"github.com/louloulibs/pipelink/internal/display"
	"github.com/louloulibs/pipelink/internal/linker"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate <config.toml>",
	Short: "Check that all source paths in a config exist",
	Long:  "Reads a TOML configuration and verifies that every source file and directory exists, without creating any symlinks.",
	Args:  cobra.ExactArgs(1),
	RunE:  runValidate,
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

func runValidate(cmd *cobra.Command, args []string) error {
	configPath := args[0]

	entries, err := config.LoadConfig(configPath)
	if err != nil {
		display.ErrorMsg(fmt.Sprintf("Failed to load config: %v", err))
		return err
	}

	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)

	anyMissing := false
	for name, e := range entries {
		srcFiles := e.Source.File.Strings()
		ok := linker.CheckPath(e.Metadata.Type, e.Source.Directory, srcFiles, e.Source.Task)

		if ok {
			green.Fprintf(os.Stderr, "  ✓ ")
		} else {
			red.Fprintf(os.Stderr, "  ✗ ")
			anyMissing = true
		}
		fmt.Fprintf(os.Stderr, "%s", name)

		srcDir := filepath.Join(e.Source.Task, e.Source.Directory)
		fmt.Fprintf(os.Stderr, "  (%s)\n", srcDir)
	}

	fmt.Fprintln(os.Stderr)
	if anyMissing {
		red.Fprintln(os.Stderr, "Some sources are missing.")
		return fmt.Errorf("validation failed")
	}
	green.Fprintln(os.Stderr, "All sources present.")
	return nil
}
