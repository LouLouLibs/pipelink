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
		path  string
		isDir bool
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
