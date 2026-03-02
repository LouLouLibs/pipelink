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
	if got1 != "c" || got2 != "c" {
		t.Errorf("identical paths: got (%q, %q), want (\"c\", \"c\")", got1, got2)
	}
}
