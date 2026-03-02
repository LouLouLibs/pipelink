package linker

import (
	"fmt"
	"os"
	"path/filepath"
)

// CheckPath verifies that source paths exist.
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
	if linkType == "directory" {
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
		if _, err := os.Lstat(target); err == nil {
			if err := os.Remove(target); err != nil {
				return fmt.Errorf("removing existing target %s: %w", target, err)
			}
		}
	}

	if err := os.Symlink(source, target); err != nil {
		return fmt.Errorf("creating symlink %s -> %s: %w", target, source, err)
	}

	return nil
}

// EnsureDir creates a directory and all parents if they don't exist.
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}
