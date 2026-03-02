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
	if len(tgtFiles) != len(srcFiles) {
		t.Errorf("target file should default to source file: got target=%v source=%v", tgtFiles, srcFiles)
	}
	if len(tgtFiles) > 0 && tgtFiles[0] != "data.csv" {
		t.Errorf("expected target file 'data.csv', got %q", tgtFiles[0])
	}
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
