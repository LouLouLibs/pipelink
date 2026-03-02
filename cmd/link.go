package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/loulou/pipelink/internal/config"
	"github.com/loulou/pipelink/internal/display"
	"github.com/loulou/pipelink/internal/linker"
	"github.com/spf13/cobra"
)

var dryRun bool

var linkCmd = &cobra.Command{
	Use:   "link <config.toml>",
	Short: "Create symlinks from a TOML config file",
	Long:  "Reads a TOML configuration and creates symbolic links for each entry, wiring up data dependencies between project directories.",
	Args:  cobra.ExactArgs(1),
	RunE:  runLink,
}

func init() {
	linkCmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "print what would be done without creating symlinks")
	rootCmd.AddCommand(linkCmd)
}

// resolveDir returns dir as-is if it is absolute, otherwise joins it with base.
func resolveDir(base, dir string) string {
	if filepath.IsAbs(dir) {
		return dir
	}
	return filepath.Join(base, dir)
}

func runLink(cmd *cobra.Command, args []string) error {
	configPath := args[0]
	cwd, _ := os.Getwd()

	display.Header(configPath)

	entries, err := config.LoadConfig(configPath)
	if err != nil {
		display.ErrorMsg(fmt.Sprintf("Failed to load config: %v", err))
		return err
	}

	if dryRun {
		fmt.Fprintf(os.Stderr, "Dry run ... no link will be established\n\n")
	}

	// Phase 1: check all paths, separate valid from missing
	var missing []string
	valid := make(map[string]config.Entry)
	for name, e := range entries {
		srcFiles := e.Source.File.Strings()
		if linker.CheckPath(e.Metadata.Type, e.Source.Directory, srcFiles, e.Source.Task) {
			valid[name] = e
		} else {
			desc := name + " ... " + filepath.Join(e.Source.Task, e.Source.Directory)
			if len(srcFiles) > 0 {
				desc += "/" + srcFiles[0]
			}
			missing = append(missing, desc)
		}
	}

	display.EntryCount(len(valid))

	// Phase 2: process each valid entry
	created := 0
	var linkErrors []error
	for name, e := range valid {
		display.EntryHeader(name, e.Metadata.Type, e.Metadata.Description)

		srcTask := e.Source.Task

		switch e.Metadata.Type {
		case "file":
			srcFiles := e.Source.File.Strings()
			tgtFiles := e.Target.File.Strings()
			if len(srcFiles) == 0 {
				continue
			}
			srcPath := filepath.Join(srcTask, e.Source.Directory, srcFiles[0])
			tgtFile := srcFiles[0]
			if len(tgtFiles) > 0 {
				tgtFile = tgtFiles[0]
			}
			tgtDir := resolveDir(cwd, e.Target.Directory)
			tgtPath := filepath.Join(tgtDir, tgtFile)

			if err := linker.EnsureDir(tgtDir); err != nil && verbose {
				display.VerboseMsg(fmt.Sprintf("creating directory: %s", tgtDir))
			}

			display.LinkPair(tgtPath, srcPath)
			if dryRun {
				display.DryRunNote()
			} else {
				if err := linker.CreateSymlink(srcPath, tgtPath, "file"); err != nil {
					display.ErrorMsg(err.Error())
					linkErrors = append(linkErrors, err)
				} else {
					created++
				}
			}

		case "directory":
			srcPath := filepath.Join(srcTask, e.Source.Directory)
			tgtPath := resolveDir(cwd, e.Target.Directory)

			parentDir := filepath.Dir(tgtPath)
			if err := linker.EnsureDir(parentDir); err != nil && verbose {
				display.VerboseMsg(fmt.Sprintf("creating directory: %s", parentDir))
			}

			display.LinkPair(tgtPath, srcPath)
			if dryRun {
				display.DryRunNote()
			} else {
				if err := linker.CreateSymlink(srcPath, tgtPath, "directory"); err != nil {
					display.ErrorMsg(err.Error())
					linkErrors = append(linkErrors, err)
				} else {
					created++
				}
			}

		case "files":
			srcFiles := e.Source.File.Strings()
			tgtFiles := e.Target.File.Strings()
			if len(srcFiles) != len(tgtFiles) {
				display.ErrorMsg("source and target file lists must have the same length")
				continue
			}

			// Ensure target directories exist
			tgtBase := resolveDir(cwd, e.Target.Directory)
			tgtDirs := make(map[string]bool)
			for _, tf := range tgtFiles {
				d := filepath.Dir(filepath.Join(tgtBase, tf))
				tgtDirs[d] = true
			}
			for d := range tgtDirs {
				linker.EnsureDir(d)
			}

			for i, sf := range srcFiles {
				srcPath := filepath.Join(srcTask, e.Source.Directory, sf)
				tgtPath := filepath.Join(tgtBase, tgtFiles[i])

				if i < 5 {
					display.LinkPair(tgtPath, srcPath)
				}
				if dryRun {
					if i < 5 {
						display.DryRunNote()
					}
				} else {
					if err := linker.CreateSymlink(srcPath, tgtPath, "file"); err != nil {
						display.ErrorMsg(err.Error())
						linkErrors = append(linkErrors, err)
					} else {
						created++
					}
				}
			}
			if len(srcFiles) > 5 {
				fmt.Fprintf(os.Stderr, "         ... and %d more\n", len(srcFiles)-5)
			}
		}

		display.EntryEnd()
	}

	// Phase 3: print warnings and summary
	display.MissingWarning(missing)
	display.Summary(created, len(missing))

	if len(linkErrors) > 0 {
		return fmt.Errorf("%d symlink(s) failed", len(linkErrors))
	}
	return nil
}
