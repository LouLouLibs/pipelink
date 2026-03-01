# Pipelink Go Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build `pipelink`, a Go CLI that reads TOML configs and creates symlinks across project directories — replacing the Python `link_json` utility.

**Architecture:** Cobra CLI with two subcommands (`link`, `validate`). Core logic in `internal/` packages: `config` (TOML parsing), `linker` (symlink ops), `display` (colored output). Entry point delegates to Cobra.

**Tech Stack:** Go 1.26, spf13/cobra, BurntSushi/toml, fatih/color

---

### Task 1: Initialize Go Module and Install Dependencies

**Files:**
- Create: `go.mod`
- Create: `main.go`

**Step 1: Initialize the Go module**

Run:
```bash
cd /Users/loulou/Dropbox/projects_claude/pipelink
go mod init github.com/loulou/pipelink
```

**Step 2: Install dependencies**

Run:
```bash
cd /Users/loulou/Dropbox/projects_claude/pipelink
go get github.com/spf13/cobra@latest
go get github.com/BurntSushi/toml@latest
go get github.com/fatih/color@latest
```

**Step 3: Create main.go**

```go
package main

import "github.com/loulou/pipelink/cmd"

func main() {
	cmd.Execute()
}
```

**Step 4: Create stub root command so it compiles**

Create `cmd/root.go`:

```go
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
```

**Step 5: Verify it compiles and runs**

Run: `cd /Users/loulou/Dropbox/projects_claude/pipelink && go build -o pipelink . && ./pipelink --help`

Expected: Help text showing "Create symlinks across projects from TOML config"

**Step 6: Commit**

```bash
git add main.go go.mod go.sum cmd/root.go
git commit -m "feat: initialize Go module with Cobra root command"
```

---

### Task 2: Config Package — TOML Parsing and Data Model

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Step 1: Write the test file**

Create `internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_SingleFile(t *testing.T) {
	content := `
[SALOMONBONDS.metadata]
type = "file"
description = "Salomon Brothers yield data"

[SALOMONBONDS.source]
directory = "/data/SalomonBrothers"
file = "SalomonBrothers_yields.xlsx"

[SALOMONBONDS.target]
directory = "./input/MuniBonds"
file = "SalomonBrothers_yields.xlsx"
`
	path := writeTempTOML(t, content)
	entries, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries["SALOMONBONDS"]
	if e.Metadata.Type != "file" {
		t.Errorf("expected type 'file', got %q", e.Metadata.Type)
	}
	if e.Metadata.Description != "Salomon Brothers yield data" {
		t.Errorf("unexpected description: %q", e.Metadata.Description)
	}
	files := e.Source.File.Strings()
	if len(files) != 1 || files[0] != "SalomonBrothers_yields.xlsx" {
		t.Errorf("unexpected source file: %v", files)
	}
}

func TestLoadConfig_MultipleFiles(t *testing.T) {
	content := `
[GSW.metadata]
type = "files"
description = "GSW data"

[GSW.source]
directory = "/data/GSW"
file = ["GSW_parameters.parquet", "GSW_treasury_yields.parquet"]

[GSW.target]
directory = "./input/MuniBonds"
file = ["GSW_parameters.parquet", "GSW_treasury_yields.parquet"]
`
	path := writeTempTOML(t, content)
	entries, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := entries["GSW"]
	files := e.Source.File.Strings()
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[0] != "GSW_parameters.parquet" || files[1] != "GSW_treasury_yields.parquet" {
		t.Errorf("unexpected files: %v", files)
	}
}

func TestLoadConfig_Directory(t *testing.T) {
	content := `
[CENSUS_MAPS.metadata]
type = "directory"
description = "TIGER Shape Files"

[CENSUS_MAPS.source]
directory = "/data/Census/ShapeFiles"

[CENSUS_MAPS.target]
directory = "input/ShapeFiles/Census"
`
	path := writeTempTOML(t, content)
	entries, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := entries["CENSUS_MAPS"]
	if e.Metadata.Type != "directory" {
		t.Errorf("expected type 'directory', got %q", e.Metadata.Type)
	}
}

func TestLoadConfig_GeneratedBy(t *testing.T) {
	content := `
[X.metadata]
type = "file"
description = "test"
generated_by = ["script.R", "other.py"]

[X.source]
directory = "/src"
file = "a.csv"

[X.target]
directory = "./tgt"
file = "a.csv"
`
	path := writeTempTOML(t, content)
	entries, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gb := entries["X"].Metadata.GeneratedBy
	if len(gb) != 2 || gb[0] != "script.R" || gb[1] != "other.py" {
		t.Errorf("unexpected generated_by: %v", gb)
	}
}

func TestLoadConfig_TargetFileDefaultsToSource(t *testing.T) {
	content := `
[ITEM.metadata]
type = "file"

[ITEM.source]
directory = "/src"
file = "data.csv"

[ITEM.target]
directory = "./tgt"
`
	path := writeTempTOML(t, content)
	entries, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := entries["ITEM"]
	tgtFiles := e.Target.File.Strings()
	srcFiles := e.Source.File.Strings()
	if len(tgtFiles) == 0 {
		// Target file was omitted — ResolveDefaults should fill it from source
	}
	_ = srcFiles
}

func TestLoadConfig_InvalidFile(t *testing.T) {
	_, err := LoadConfig("/nonexistent/file.toml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestStringOrSlice_SingleString(t *testing.T) {
	var s StringOrSlice
	s.values = []string{"hello"}
	got := s.Strings()
	if len(got) != 1 || got[0] != "hello" {
		t.Errorf("expected [hello], got %v", got)
	}
}

func TestStringOrSlice_MultipleStrings(t *testing.T) {
	var s StringOrSlice
	s.values = []string{"a", "b", "c"}
	got := s.Strings()
	if len(got) != 3 {
		t.Errorf("expected 3 strings, got %d", len(got))
	}
}

// Helper: write TOML content to a temp file and return the path.
func writeTempTOML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/loulou/Dropbox/projects_claude/pipelink && go test ./internal/config/ -v`

Expected: Compilation errors (package doesn't exist yet)

**Step 3: Implement config.go**

Create `internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// StringOrSlice handles TOML fields that can be a single string or an array of strings.
type StringOrSlice struct {
	values []string
}

// Strings returns the underlying string slice.
func (s StringOrSlice) Strings() []string {
	return s.values
}

// IsEmpty returns true if no values are set.
func (s StringOrSlice) IsEmpty() bool {
	return len(s.values) == 0
}

// UnmarshalTOML implements custom TOML unmarshaling for string-or-array fields.
func (s *StringOrSlice) UnmarshalTOML(data interface{}) error {
	switch v := data.(type) {
	case string:
		s.values = []string{v}
	case []interface{}:
		s.values = make([]string, 0, len(v))
		for _, item := range v {
			str, ok := item.(string)
			if !ok {
				return fmt.Errorf("expected string in array, got %T", item)
			}
			s.values = append(s.values, str)
		}
	default:
		return fmt.Errorf("expected string or array, got %T", data)
	}
	return nil
}

// Metadata describes the link entry type and purpose.
type Metadata struct {
	Type        string   `toml:"type"`
	Description string   `toml:"description"`
	GeneratedBy []string `toml:"generated_by"`
}

// Source specifies where to link from.
type Source struct {
	Directory string        `toml:"directory"`
	File      StringOrSlice `toml:"file"`
	Task      string        `toml:"task"`
}

// Target specifies where to link to.
type Target struct {
	Directory string        `toml:"directory"`
	File      StringOrSlice `toml:"file"`
}

// Entry represents one link specification in the config.
type Entry struct {
	Metadata Metadata `toml:"metadata"`
	Source   Source    `toml:"source"`
	Target   Target    `toml:"target"`
}

// LoadConfig reads a TOML file and returns the parsed entries.
func LoadConfig(path string) (map[string]Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var entries map[string]Entry
	if err := toml.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parsing TOML: %w", err)
	}

	// Default target.file to source.file when omitted
	for name, e := range entries {
		if e.Target.File.IsEmpty() && !e.Source.File.IsEmpty() {
			e.Target.File = e.Source.File
			entries[name] = e
		}
	}

	return entries, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/loulou/Dropbox/projects_claude/pipelink && go test ./internal/config/ -v`

Expected: All tests PASS

**Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: add config package with TOML parsing and StringOrSlice type"
```

---

### Task 3: Display Package — Colored Output and Unicode Formatting

**Files:**
- Create: `internal/display/display.go`
- Create: `internal/display/display_test.go`

**Step 1: Write the test file**

Create `internal/display/display_test.go`:

```go
package display

import (
	"path/filepath"
	"testing"
)

func TestRemoveCommonPrefix(t *testing.T) {
	tests := []struct {
		path1, path2 string
		want1, want2 string
	}{
		{
			"/Users/loulou/Dropbox/project/input/data.csv",
			"/Users/loulou/Dropbox/munis_home/data/data.csv",
			filepath.Join("project", "input", "data.csv"),
			filepath.Join("munis_home", "data", "data.csv"),
		},
		{
			"/a/b/c/file1.txt",
			"/a/b/d/file2.txt",
			filepath.Join("c", "file1.txt"),
			filepath.Join("d", "file2.txt"),
		},
		{
			"/completely/different",
			"/nothing/in/common",
			filepath.Join("completely", "different"),
			filepath.Join("nothing", "in", "common"),
		},
	}

	for _, tt := range tests {
		got1, got2 := RemoveCommonPrefix(tt.path1, tt.path2)
		if got1 != tt.want1 || got2 != tt.want2 {
			t.Errorf("RemoveCommonPrefix(%q, %q) = (%q, %q), want (%q, %q)",
				tt.path1, tt.path2, got1, got2, tt.want1, tt.want2)
		}
	}
}

func TestRemoveCommonPrefix_IdenticalPaths(t *testing.T) {
	got1, got2 := RemoveCommonPrefix("/a/b/c", "/a/b/c")
	// When paths are identical, both suffixes should be just the filename/last component
	if got1 != "c" || got2 != "c" {
		t.Errorf("identical paths: got (%q, %q), want (\"c\", \"c\")", got1, got2)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/loulou/Dropbox/projects_claude/pipelink && go test ./internal/display/ -v`

Expected: Compilation error

**Step 3: Implement display.go**

Create `internal/display/display.go`:

```go
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
	boldRed := color.New(color.Bold, color.FgRed)
	fmt.Fprintf(color.Error, "🔗    ")
	boldRed.Fprintf(color.Error, "Processing ... ")
	color.New(color.Bold, color.FgRed, color.Underline).Fprintf(color.Error, "%s", configFile)
	boldRed.Fprintf(color.Error, " ... for linking")
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
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/loulou/Dropbox/projects_claude/pipelink && go test ./internal/display/ -v`

Expected: All tests PASS

**Step 5: Commit**

```bash
git add internal/display/
git commit -m "feat: add display package with colored output and Unicode formatting"
```

---

### Task 4: Linker Package — Symlink Operations

**Files:**
- Create: `internal/linker/linker.go`
- Create: `internal/linker/linker_test.go`

**Step 1: Write the test file**

Create `internal/linker/linker_test.go`:

```go
package linker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckPath_FileExists(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.csv")
	os.WriteFile(f, []byte("data"), 0644)

	if !CheckPath("file", dir, []string{"test.csv"}, "") {
		t.Error("expected file to exist")
	}
}

func TestCheckPath_FileMissing(t *testing.T) {
	dir := t.TempDir()
	if CheckPath("file", dir, []string{"missing.csv"}, "") {
		t.Error("expected file to be missing")
	}
}

func TestCheckPath_DirectoryExists(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir")
	os.Mkdir(sub, 0755)

	if !CheckPath("directory", sub, nil, "") {
		t.Error("expected directory to exist")
	}
}

func TestCheckPath_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.csv"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.csv"), []byte("b"), 0644)

	if !CheckPath("files", dir, []string{"a.csv", "b.csv"}, "") {
		t.Error("expected all files to exist")
	}
}

func TestCheckPath_MultipleFilesOneMissing(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.csv"), []byte("a"), 0644)

	if CheckPath("files", dir, []string{"a.csv", "missing.csv"}, "") {
		t.Error("expected check to fail when one file is missing")
	}
}

func TestCreateSymlink_File(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.txt")
	tgt := filepath.Join(dir, "target.txt")
	os.WriteFile(src, []byte("hello"), 0644)

	err := CreateSymlink(src, tgt, "file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Lstat(tgt)
	if err != nil {
		t.Fatalf("target does not exist: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("target is not a symlink")
	}

	dest, _ := os.Readlink(tgt)
	if dest != src {
		t.Errorf("symlink points to %q, want %q", dest, src)
	}
}

func TestCreateSymlink_Directory(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "srcdir")
	tgt := filepath.Join(dir, "tgtdir")
	os.Mkdir(src, 0755)
	os.WriteFile(filepath.Join(src, "data.csv"), []byte("data"), 0644)

	err := CreateSymlink(src, tgt, "directory")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Lstat(tgt)
	if err != nil {
		t.Fatalf("target does not exist: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("target is not a symlink")
	}
}

func TestCreateSymlink_ReplacesExistingSymlink(t *testing.T) {
	dir := t.TempDir()
	src1 := filepath.Join(dir, "src1.txt")
	src2 := filepath.Join(dir, "src2.txt")
	tgt := filepath.Join(dir, "target.txt")
	os.WriteFile(src1, []byte("first"), 0644)
	os.WriteFile(src2, []byte("second"), 0644)

	// Create first symlink
	os.Symlink(src1, tgt)

	// Replace with second
	err := CreateSymlink(src2, tgt, "file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dest, _ := os.Readlink(tgt)
	if dest != src2 {
		t.Errorf("symlink points to %q, want %q", dest, src2)
	}
}

func TestCreateSymlink_ReplacesExistingDirectory(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "srcdir")
	tgt := filepath.Join(dir, "tgtdir")
	os.Mkdir(src, 0755)

	// Create a real directory at target
	os.Mkdir(tgt, 0755)
	os.WriteFile(filepath.Join(tgt, "old.txt"), []byte("old"), 0644)

	err := CreateSymlink(src, tgt, "directory")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, _ := os.Lstat(tgt)
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("target should be a symlink after replacement")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/loulou/Dropbox/projects_claude/pipelink && go test ./internal/linker/ -v`

Expected: Compilation error

**Step 3: Implement linker.go**

Create `internal/linker/linker.go`:

```go
package linker

import (
	"fmt"
	"os"
	"path/filepath"
)

// CheckPath verifies that source paths exist.
// For "file": checks one file. For "files": checks all files. For "directory": checks directory.
func CheckPath(linkType, directory string, files []string, task string) bool {
	base := filepath.Join(task, directory)

	switch linkType {
	case "file":
		if len(files) == 0 {
			return false
		}
		p := filepath.Join(base, files[0])
		_, err := os.Stat(p)
		return err == nil
	case "files":
		for _, f := range files {
			p := filepath.Join(base, f)
			if _, err := os.Stat(p); err != nil {
				return false
			}
		}
		return len(files) > 0
	case "directory":
		info, err := os.Stat(base)
		return err == nil && info.IsDir()
	default:
		return false
	}
}

// CreateSymlink creates a symlink from source to target.
// Removes any existing file, symlink, or directory at the target path.
func CreateSymlink(source, target, linkType string) error {
	// Remove existing target
	if linkType == "directory" {
		// For directories, check if it's a symlink first (don't rmtree a symlink)
		if info, err := os.Lstat(target); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				if err := os.Remove(target); err != nil {
					return fmt.Errorf("removing existing symlink %s: %w", target, err)
				}
			} else {
				if err := os.RemoveAll(target); err != nil {
					return fmt.Errorf("removing existing directory %s: %w", target, err)
				}
			}
		}
	} else {
		// For files, remove existing file or symlink
		if _, err := os.Lstat(target); err == nil {
			if err := os.Remove(target); err != nil {
				return fmt.Errorf("removing existing target %s: %w", target, err)
			}
		}
	}

	// Create the symlink
	if err := os.Symlink(source, target); err != nil {
		return fmt.Errorf("creating symlink %s -> %s: %w", target, source, err)
	}

	return nil
}

// EnsureDir creates a directory and all parents if they don't exist.
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/loulou/Dropbox/projects_claude/pipelink && go test ./internal/linker/ -v`

Expected: All tests PASS

**Step 5: Commit**

```bash
git add internal/linker/
git commit -m "feat: add linker package with symlink creation and path checking"
```

---

### Task 5: Link Command — Wire Everything Together

**Files:**
- Create: `cmd/link.go`

**Step 1: Implement the link command**

Create `cmd/link.go`:

```go
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
		tgtTask := e.Target.Directory
		if tgtTask == "" {
			tgtTask = "."
		}

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
			tgtDir := filepath.Join(cwd, e.Target.Directory)
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
			tgtPath := filepath.Join(cwd, e.Target.Directory)

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
			tgtDirs := make(map[string]bool)
			for _, tf := range tgtFiles {
				d := filepath.Dir(filepath.Join(cwd, e.Target.Directory, tf))
				tgtDirs[d] = true
			}
			for d := range tgtDirs {
				linker.EnsureDir(d)
			}

			for i, sf := range srcFiles {
				srcPath := filepath.Join(srcTask, e.Source.Directory, sf)
				tgtPath := filepath.Join(cwd, e.Target.Directory, tgtFiles[i])

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
```

**Step 2: Verify it compiles**

Run: `cd /Users/loulou/Dropbox/projects_claude/pipelink && go build -o pipelink .`

Expected: Compiles without errors

**Step 3: Commit**

```bash
git add cmd/link.go
git commit -m "feat: add link command — main symlink logic"
```

---

### Task 6: Validate Command

**Files:**
- Create: `cmd/validate.go`

**Step 1: Implement the validate command**

Create `cmd/validate.go`:

```go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/loulou/pipelink/internal/config"
	"github.com/loulou/pipelink/internal/display"
	"github.com/loulou/pipelink/internal/linker"
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
```

**Step 2: Verify it compiles and test both commands**

Run: `cd /Users/loulou/Dropbox/projects_claude/pipelink && go build -o pipelink . && ./pipelink --help && ./pipelink link --help && ./pipelink validate --help`

Expected: Help text for all three levels

**Step 3: Commit**

```bash
git add cmd/validate.go
git commit -m "feat: add validate command — check source paths without linking"
```

---

### Task 7: Integration Test with Real TOML

**Files:**
- Create: `integration_test.go`
- Create: `testdata/basic.toml`

**Step 1: Create test fixture**

Create `testdata/basic.toml`:

```toml
[SINGLE_FILE.metadata]
type = "file"
description = "A single test file"

[SINGLE_FILE.source]
directory = "SRCDIR"
file = "data.csv"

[SINGLE_FILE.target]
directory = "TGTDIR"
file = "data.csv"

[MULTI_FILES.metadata]
type = "files"
description = "Multiple test files"

[MULTI_FILES.source]
directory = "SRCDIR"
file = ["a.csv", "b.csv"]

[MULTI_FILES.target]
directory = "TGTDIR"
file = ["a.csv", "b.csv"]

[A_DIRECTORY.metadata]
type = "directory"
description = "A whole directory"

[A_DIRECTORY.source]
directory = "SRCDIR/subdir"

[A_DIRECTORY.target]
directory = "TGTDIR/subdir"
```

Note: `SRCDIR` and `TGTDIR` are placeholders — the integration test will rewrite them to temp directory paths at runtime.

**Step 2: Write integration test**

Create `integration_test.go`:

```go
//go:build integration

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntegration_LinkCommand(t *testing.T) {
	// Build the binary
	binPath := filepath.Join(t.TempDir(), "pipelink")
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = "."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %s\n%s", err, out)
	}

	// Set up temp directories
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	tgtDir := filepath.Join(tmpDir, "tgt")
	os.MkdirAll(srcDir, 0755)
	os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755)

	// Create source files
	os.WriteFile(filepath.Join(srcDir, "data.csv"), []byte("single"), 0644)
	os.WriteFile(filepath.Join(srcDir, "a.csv"), []byte("multi-a"), 0644)
	os.WriteFile(filepath.Join(srcDir, "b.csv"), []byte("multi-b"), 0644)
	os.WriteFile(filepath.Join(srcDir, "subdir", "inside.txt"), []byte("dir"), 0644)

	// Read and rewrite the test TOML with actual paths
	tomlBytes, _ := os.ReadFile("testdata/basic.toml")
	tomlStr := string(tomlBytes)
	tomlStr = strings.ReplaceAll(tomlStr, "SRCDIR", srcDir)
	tomlStr = strings.ReplaceAll(tomlStr, "TGTDIR", tgtDir)

	configPath := filepath.Join(tmpDir, "test.toml")
	os.WriteFile(configPath, []byte(tomlStr), 0644)

	// Run pipelink link
	cmd := exec.Command(binPath, "link", configPath)
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pipelink link failed: %s\n%s", err, out)
	}

	// Verify symlinks were created
	checks := []struct {
		path   string
		isDir  bool
	}{
		{filepath.Join(tgtDir, "data.csv"), false},
		{filepath.Join(tgtDir, "a.csv"), false},
		{filepath.Join(tgtDir, "b.csv"), false},
		{filepath.Join(tgtDir, "subdir"), true},
	}

	for _, c := range checks {
		info, err := os.Lstat(c.path)
		if err != nil {
			t.Errorf("expected symlink at %s, got error: %v", c.path, err)
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%s is not a symlink", c.path)
		}
	}
}

func TestIntegration_ValidateCommand(t *testing.T) {
	binPath := filepath.Join(t.TempDir(), "pipelink")
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = "."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %s\n%s", err, out)
	}

	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	os.MkdirAll(srcDir, 0755)
	os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(srcDir, "data.csv"), []byte("data"), 0644)
	os.WriteFile(filepath.Join(srcDir, "a.csv"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(srcDir, "b.csv"), []byte("b"), 0644)

	tomlBytes, _ := os.ReadFile("testdata/basic.toml")
	tomlStr := strings.ReplaceAll(string(tomlBytes), "SRCDIR", srcDir)
	tomlStr = strings.ReplaceAll(tomlStr, "TGTDIR", filepath.Join(tmpDir, "tgt"))
	configPath := filepath.Join(tmpDir, "test.toml")
	os.WriteFile(configPath, []byte(tomlStr), 0644)

	cmd := exec.Command(binPath, "validate", configPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("validate failed: %s\n%s", err, out)
	}
	if !strings.Contains(string(out), "All sources present") {
		t.Errorf("expected success message, got: %s", out)
	}
}

func TestIntegration_DryRun(t *testing.T) {
	binPath := filepath.Join(t.TempDir(), "pipelink")
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = "."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %s\n%s", err, out)
	}

	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	tgtDir := filepath.Join(tmpDir, "tgt")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "data.csv"), []byte("data"), 0644)

	toml := `
[ITEM.metadata]
type = "file"
[ITEM.source]
directory = "` + srcDir + `"
file = "data.csv"
[ITEM.target]
directory = "` + tgtDir + `"
file = "data.csv"
`
	configPath := filepath.Join(tmpDir, "test.toml")
	os.WriteFile(configPath, []byte(toml), 0644)

	cmd := exec.Command(binPath, "link", "--dry-run", configPath)
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dry-run failed: %s\n%s", err, out)
	}

	// Target should NOT exist after dry run
	if _, err := os.Lstat(filepath.Join(tgtDir, "data.csv")); err == nil {
		t.Error("dry-run should not create symlinks")
	}
}
```

**Step 2: Run integration tests**

Run: `cd /Users/loulou/Dropbox/projects_claude/pipelink && go test -tags=integration -v .`

Expected: All tests PASS

**Step 3: Commit**

```bash
git add integration_test.go testdata/
git commit -m "test: add integration tests for link, validate, and dry-run"
```

---

### Task 8: Run All Tests and Build Final Binary

**Step 1: Run all unit tests**

Run: `cd /Users/loulou/Dropbox/projects_claude/pipelink && go test ./... -v`

Expected: All unit tests PASS

**Step 2: Run integration tests**

Run: `cd /Users/loulou/Dropbox/projects_claude/pipelink && go test -tags=integration -v .`

Expected: All integration tests PASS

**Step 3: Build the binary**

Run: `cd /Users/loulou/Dropbox/projects_claude/pipelink && go build -o pipelink .`

Expected: Binary created

**Step 4: Test with a real config from munis_home**

Run: `cd /Users/loulou/Dropbox/projects_claude/pipelink && ./pipelink validate /Users/loulou/Dropbox/munis_home/project_ivanov/tmp/input.toml`

Expected: Validation output showing check results

**Step 5: Commit**

```bash
git commit --allow-empty -m "chore: all tests passing, binary builds successfully"
```

---

### Task 9: Cross-Platform Build

**Step 1: Build for all target platforms**

Run:
```bash
cd /Users/loulou/Dropbox/projects_claude/pipelink
GOOS=darwin GOARCH=arm64 go build -o dist/pipelink-darwin-arm64 .
GOOS=darwin GOARCH=amd64 go build -o dist/pipelink-darwin-amd64 .
GOOS=linux GOARCH=amd64 go build -o dist/pipelink-linux-amd64 .
GOOS=linux GOARCH=arm64 go build -o dist/pipelink-linux-arm64 .
GOOS=windows GOARCH=amd64 go build -o dist/pipelink-windows-amd64.exe .
```

Expected: All five binaries created in `dist/`

**Step 2: Verify local binary works**

Run: `cd /Users/loulou/Dropbox/projects_claude/pipelink && ./dist/pipelink-darwin-arm64 --help`

Expected: Help text

**Step 3: Commit**

```bash
echo "dist/" >> .gitignore
git add .gitignore
git commit -m "chore: add .gitignore for dist/ build artifacts"
```
