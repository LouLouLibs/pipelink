package display

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
)

var (
	bold      = color.New(color.Bold)
	boldRed   = color.New(color.Bold, color.FgRed)
	green     = color.New(color.FgGreen)
	darkGreen = color.New(color.FgGreen, color.Italic)
	blue      = color.New(color.FgBlue, color.Italic)
	dimItalic = color.New(color.Faint, color.Italic)
	yellow    = color.New(color.FgYellow, color.Italic)
)

// Header prints the processing header.
func Header(configFile string) {
	fmt.Fprintf(color.Error, "🔗    ")
	color.New(color.Bold, color.FgRed).Fprintf(color.Error, "Processing ... ")
	color.New(color.Bold, color.FgRed, color.Underline).Fprintf(color.Error, "%s", configFile)
	color.New(color.Bold, color.FgRed).Fprintf(color.Error, " ... for linking")
	fmt.Fprintf(color.Error, "    🔗\n\n")
}

// EntryCount prints how many entries will be processed.
func EntryCount(n int) {
	fmt.Fprintf(color.Error, "      %d files to process\n\n", n)
}

// EntryHeader prints the entry name and type annotation.
func EntryHeader(name, linkType, description string) {
	bold.Fprintf(color.Error, "Linking  %s ", name)
	switch linkType {
	case "files":
		darkGreen.Fprintf(color.Error, "    (multiple files)")
	case "directory":
		darkGreen.Fprintf(color.Error, "    (directory)")
	}
	fmt.Fprintln(color.Error)
	if description != "" {
		fmt.Fprintf(color.Error, "         ")
		color.New(color.Italic).Fprintf(color.Error, "%s", description)
		fmt.Fprintln(color.Error)
	}
}

// LinkPair prints one target/source pair with Unicode box-drawing arrows.
func LinkPair(tgtPath, srcPath string) {
	tgtShort, srcShort := RemoveCommonPrefix(tgtPath, srcPath)
	fmt.Fprintf(color.Error, "Target:  ")
	boldRed.Fprintf(color.Error, "┌─▶")
	blue.Fprintf(color.Error, " %s", tgtShort)
	fmt.Fprintln(color.Error)
	fmt.Fprintf(color.Error, "Source:  ")
	boldRed.Fprintf(color.Error, "└──")
	green.Fprintf(color.Error, " %s", srcShort)
	fmt.Fprintln(color.Error)
}

// DryRunNote prints the dry-run annotation.
func DryRunNote() {
	fmt.Fprintf(color.Error, "             (")
	color.New(color.Underline).Fprintf(color.Error, "dry-run")
	fmt.Fprintln(color.Error, ")")
}

// EntryEnd prints a blank line after an entry.
func EntryEnd() {
	fmt.Fprintln(color.Error)
}

// MissingWarning prints the warning about missing source files.
func MissingWarning(missing []string) {
	if len(missing) == 0 {
		return
	}
	fmt.Fprintf(color.Error, "  ⚠️    Some input files are missing (filtering them out)\n")
	for _, m := range missing {
		dimItalic.Fprintf(color.Error, "          %s\n", m)
	}
}

// Summary prints the final summary line.
func Summary(created, skipped int) {
	if skipped == 0 {
		green.Fprintf(color.Error, "✓ %d links created\n", created)
	} else {
		yellow.Fprintf(color.Error, "✓ %d links created, %d sources missing (skipped)\n", created, skipped)
	}
	fmt.Fprintln(color.Error)
}

// VerboseMsg prints a message only in verbose mode.
func VerboseMsg(msg string) {
	yellow.Fprintf(color.Error, "  %s\n", msg)
}

// ErrorMsg prints an error message.
func ErrorMsg(msg string) {
	color.New(color.Bold, color.FgRed).Fprintf(color.Error, "ERROR: ")
	fmt.Fprintln(color.Error, msg)
}

// RemoveCommonPrefix strips the shared directory prefix from two paths
// and returns the unique suffixes for compact display.
func RemoveCommonPrefix(path1, path2 string) (string, string) {
	parts1 := splitPath(path1)
	parts2 := splitPath(path2)

	commonLen := 0
	for i := 0; i < len(parts1) && i < len(parts2); i++ {
		if parts1[i] == parts2[i] {
			commonLen++
		} else {
			break
		}
	}

	// Ensure at least the last component remains in each suffix.
	if commonLen >= len(parts1) {
		commonLen = len(parts1) - 1
	}
	if commonLen >= len(parts2) {
		commonLen = len(parts2) - 1
	}

	suffix1 := filepath.Join(parts1[commonLen:]...)
	suffix2 := filepath.Join(parts2[commonLen:]...)
	return suffix1, suffix2
}

// splitPath splits an absolute or relative path into its components.
func splitPath(p string) []string {
	p = filepath.Clean(p)
	var parts []string
	for {
		dir, file := filepath.Split(p)
		if file != "" {
			parts = append([]string{file}, parts...)
		}
		dir = strings.TrimRight(dir, string(filepath.Separator))
		if dir == "" || dir == p {
			if dir != "" {
				parts = append([]string{dir}, parts...)
			}
			break
		}
		p = dir
	}
	return parts
}
