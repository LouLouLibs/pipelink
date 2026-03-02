# pipelink

Create symbolic links across project directories from a TOML configuration file. Designed for monorepos and data pipelines where one project's output is another project's input — pipelink wires them together with symlinks, avoiding data duplication.

## Installation

### Download a release

Download the latest binary for your platform from the [releases page](https://github.com/louloulibs/pipelink/releases).

```bash
# macOS (Apple Silicon)
curl -L https://github.com/louloulibs/pipelink/releases/latest/download/pipelink_darwin_arm64.tar.gz | tar xz

# macOS (Intel)
curl -L https://github.com/louloulibs/pipelink/releases/latest/download/pipelink_darwin_amd64.tar.gz | tar xz

# Linux (x86_64)
curl -L https://github.com/louloulibs/pipelink/releases/latest/download/pipelink_linux_amd64.tar.gz | tar xz
```

### Build from source

Requires [Go 1.22+](https://go.dev/dl/).

```bash
git clone https://github.com/louloulibs/pipelink.git
cd pipelink
go build -ldflags="-s -w" -o pipelink .
```

The `-ldflags="-s -w"` flag strips debug symbols for a smaller binary (~4 MB → ~3 MB).

## Usage

### `pipelink link`

Read a TOML config and create all symlinks:

```bash
pipelink link input.toml
```

Preview what would happen without creating anything:

```bash
pipelink link --dry-run input.toml
```

### `pipelink validate`

Check that all source files and directories exist, without creating symlinks:

```bash
pipelink validate input.toml
```

Exits with code 0 if all sources are present, 1 if any are missing.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--dry-run` | `-d` | Print actions without executing (link only) |
| `--verbose` | `-v` | Show additional output |
| `--help` | `-h` | Help for any command |

## Configuration

Pipelink reads a TOML file where each top-level table defines one link. Each entry has three sections: `metadata`, `source`, and `target`.

### Link a single file

```toml
[SALOMON_YIELDS.metadata]
type = "file"
description = "Salomon Brothers yield data"

[SALOMON_YIELDS.source]
directory = "/data/SalomonBrothers"
file = "SalomonBrothers_yields.xlsx"

[SALOMON_YIELDS.target]
directory = "./input/MuniBonds"
file = "SalomonBrothers_yields.xlsx"
```

### Link multiple files

```toml
[GSW.metadata]
type = "files"
description = "GSW interest rate model parameters"

[GSW.source]
directory = "/data/FederalReserve/GSW"
file = [
    "GSW_parameters.parquet",
    "GSW_treasury_yields.parquet",
]

[GSW.target]
directory = "./input/MuniBonds"
file = [
    "GSW_parameters.parquet",
    "GSW_treasury_yields.parquet",
]
```

Source and target file arrays must have the same length. Each file at index `i` in source is linked to the file at index `i` in target.

### Link a directory

```toml
[CENSUS_MAPS.metadata]
type = "directory"
description = "TIGER/Line shapefiles"

[CENSUS_MAPS.source]
directory = "/data/Census/ShapeFiles"

[CENSUS_MAPS.target]
directory = "input/ShapeFiles/Census"
```

### Config reference

**`metadata` (required)**

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | `"file"`, `"files"`, or `"directory"` |
| `description` | string | Optional human-readable description |
| `generated_by` | string[] | Optional list of scripts that produce the source |

**`source` (required)**

| Field | Type | Description |
|-------|------|-------------|
| `directory` | string | Absolute path to the source directory |
| `file` | string or string[] | Filename(s) within the directory. Omit for `type = "directory"` |
| `task` | string | Optional path prefix prepended to `directory` |

**`target` (required)**

| Field | Type | Description |
|-------|------|-------------|
| `directory` | string | Path to the target directory (relative to working directory, or absolute) |
| `file` | string or string[] | Target filename(s). Defaults to source filenames if omitted |

## Output

Pipelink prints colored output showing each link with Unicode arrows:

```
🔗    Processing ... input.toml ... for linking    🔗

      4 files to process

Linking  GSW     (multiple files)
         GSW interest rate model parameters
Target:  ┌─▶ input/MuniBonds/GSW_parameters.parquet
Source:  └── data/FederalReserve/GSW/GSW_parameters.parquet
Target:  ┌─▶ input/MuniBonds/GSW_treasury_yields.parquet
Source:  └── data/FederalReserve/GSW/GSW_treasury_yields.parquet

✓ 4 links created
```

Missing source files are filtered out with a warning and the remaining links are still created.

## Snakemake integration

Use pipelink as a rule in your Snakemake pipeline to establish data dependencies before running analysis:

```python
rule link_inputs:
    input:
        config="input.toml",
    output:
        touch(".links_created"),
    shell:
        "pipelink link {input.config} && touch {output}"
```

## Nickel configuration

Pipelink pairs well with [Nickel](https://nickel-lang.org/) for type-safe, validated configuration. Instead of writing TOML by hand, define links in a `.ncl` file with contracts that enforce correct structure, then export to TOML.

A typical `input.ncl` in a project directory:

```nickel
let link_contracts = import "../utilities/config/nickel/link_contracts.ncl" in
let
    Link = link_contracts.link,
    serialize_records = link_contracts.serialize_records
in

{
  MUNI_AGG_BONDS | Link = 'files {
    source = {
      file = ["TOWN_AGG_bond_issuance.csv.gz",
              "COUNTY_AGG_bond_issuance.csv.gz",
              "STATE_AGG_bond_issuance.csv.gz"],
      directory = "/data/import_MuniBonds/output",
    },
    target = { directory = "./input/MuniBonds" },
    metadata = {
      generated_by = ["import_MERGENT_state.R"],
      description = "Aggregate bond issuances",
    },
  },

  SALOMONBONDS | Link = 'file {
    source = {
      file = "SalomonBrothers_yields.xlsx",
      directory = "/data/PrivateData/SalomonBrothers",
    },
    target = { directory = "./input/MuniBonds" },
  },

  GSW | Link = 'files {
    source = {
      file = ["GSW_parameters.parquet", "GSW_treasury_yields.parquet"],
      directory = "/data/FederalReserve/GSW",
    },
    target = { directory = "./input/MuniBonds" },
  },
}
|> serialize_records
```

The `Link` contract validates each entry as one of three enum variants (`'file`, `'files`, `'dir`), and `serialize_records` flattens the structure into the TOML schema pipelink expects. Target filenames default to source filenames when omitted.

Export to TOML and link in one step:

```bash
nickel export input.ncl --format toml > tmp/input.toml
pipelink link tmp/input.toml
```

Or as a Snakemake rule:

```python
rule link_inputs:
    input:
        config="input.ncl",
    output:
        toml="tmp/input.toml",
        stamp=touch(".links_created"),
    shell:
        """
        nickel export {input.config} --format toml > {output.toml}
        pipelink link {output.toml}
        """
```
