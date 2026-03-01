# Pipelink: Go Rewrite of link_json

## Purpose

Rewrite the Python `link_json` utility as a standalone Go binary called `pipelink`. Primary goals: single static binary, cross-platform distribution (macOS, Linux, Windows). No Python/pip dependency.

## What It Does

Reads a TOML config file that declares symlink specifications between directories across projects. Creates symlinks to wire up data dependencies (e.g., one project's output becomes another's input) without duplicating files.

## Project Structure

```
pipelink/
├── main.go                     # Entry point
├── go.mod
├── go.sum
├── cmd/
│   ├── root.go                 # Root cobra command, global flags
│   ├── link.go                 # pipelink link <config.toml>
│   └── validate.go             # pipelink validate <config.toml>
├── internal/
│   ├── config/
│   │   └── config.go           # TOML parsing, config structs
│   ├── linker/
│   │   └── linker.go           # Symlink creation logic
│   └── display/
│       └── display.go          # Colored output, Unicode formatting
└── docs/
    └── plans/
```

## Dependencies

- `github.com/spf13/cobra` — CLI framework with subcommands
- `github.com/BurntSushi/toml` — TOML parsing
- `github.com/fatih/color` — Colored terminal output (auto-detects piped output)

## Config Format

TOML only. Same schema as the existing Python version. Three link types: `file`, `files`, `directory`.

```toml
[GSW.metadata]
type = "files"
description = "GSW interest rate model data"

[GSW.source]
directory = "/path/to/source"
file = ["GSW_parameters.parquet", "GSW_treasury_yields.parquet"]

[GSW.target]
directory = "./input/MuniBonds"
file = ["GSW_parameters.parquet", "GSW_treasury_yields.parquet"]
```

### Go Data Model

```go
type Entry struct {
    Metadata Metadata
    Source   Source
    Target   Target
}

type Metadata struct {
    Type        string
    Description string
    GeneratedBy []string `toml:"generated_by"`
}

type Source struct {
    Directory string
    File      StringOrSlice  // custom type: string or []string
    Task      string
}

type Target struct {
    Directory string
    File      StringOrSlice  // defaults to source.File if omitted
}
```

`StringOrSlice` handles the TOML field being either a single string or an array.

## CLI Interface

```
pipelink link <config.toml> [flags]
pipelink validate <config.toml> [flags]

Global flags:
  --verbose, -v    Additional output
  --help, -h       Help

Link flags:
  --dry-run, -d    Print what would be done without creating symlinks
```

### `pipelink link` Behavior

1. Parse TOML config.
2. Validate all source paths exist. Filter out missing sources with a warning.
3. For each entry:
   - Create target directory if needed (`os.MkdirAll`).
   - Remove existing target (symlink, file, or directory).
   - Create symlink via `os.Symlink(source, target)`.
4. Print summary line.

### `pipelink validate` Behavior

1. Parse TOML config.
2. Check every source path exists.
3. Print report with green/red indicators.
4. Exit 0 if all found, exit 1 if any missing.

## Output Formatting

Preserves the Unicode box-drawing arrows from the Python version:

```
Linking  GSW     (multiple files)
         GSW interest rate model data
Target:  ┌─▶ input/MuniBonds/GSW_parameters.parquet
Source:  └── data/.../GSW/GSW_parameters.parquet
Target:  ┌─▶ input/MuniBonds/GSW_treasury_yields.parquet
Source:  └── data/.../GSW/GSW_treasury_yields.parquet
```

- `┌─▶` and `└──` in bold red
- Target path in italic blue
- Source path in green
- Entry name in bold, type annotation in italic dark green
- Common path prefix stripped for readability
- For `files` type: first 5 pairs shown, rest elided
- Dry-run: `(dry-run)` annotation per entry
- Missing sources: dimmed italic warning at end
- Summary line at end: `✓ N links created, M sources missing (skipped)`

## Error Handling

- Missing config file: clear error, exit 1
- Invalid TOML: parse error with detail, exit 1
- Missing source: listed in warning, filtered out, continues processing
- Symlink failure: error message, continues with remaining entries, exit 1 at end

## Cross-Platform Notes

- `os.Symlink` works on all platforms. Windows may need developer mode or elevated privileges for symlinks.
- `fatih/color` auto-disables color when stdout is not a terminal.
- Path handling via `filepath` package for OS-appropriate separators.
