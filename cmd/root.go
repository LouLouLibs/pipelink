package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var verbose bool

var rootCmd = &cobra.Command{
	Use:   "pipelink",
	Short: "Create symlinks across projects from TOML config",
	Long:  "Pipelink reads a TOML configuration file and creates symbolic links to wire up data dependencies between project directories.",
	// Errors are reported once by the commands / Execute; don't repeat them
	// or print usage on runtime failures.
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "additional output")
}
