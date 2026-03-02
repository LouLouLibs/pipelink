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

	os.Symlink(src1, tgt)

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
