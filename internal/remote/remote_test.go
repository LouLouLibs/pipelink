package remote

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func hexsum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func TestCachePath(t *testing.T) {
	root := "/cache"
	cases := []struct {
		uri, want string
	}{
		{"s3-msi:bucket/a/b.parquet", "/cache/s3-msi/bucket/a/b.parquet"},
		{":local:/tmp/x/y.csv", "/cache/_local/tmp/x/y.csv"},
		{"https://data.example.org/files/abc/file?x=1", "/cache/data.example.org/files/abc/file"},
		{"http://host:8080/p/q.zip", "/cache/host_8080/p/q.zip"},
	}
	for _, c := range cases {
		got, err := CachePath(root, c.uri)
		if err != nil {
			t.Errorf("CachePath(%q): unexpected error %v", c.uri, err)
			continue
		}
		if got != c.want {
			t.Errorf("CachePath(%q) = %q, want %q", c.uri, got, c.want)
		}
	}
}

func TestCachePath_Rejects(t *testing.T) {
	for _, uri := range []string{
		"s3:bucket/../../etc/passwd",
		"https://host/../x",
		"no-colon-here",
		"s3:",
		"https://host/",
	} {
		if _, err := CachePath("/cache", uri); err == nil {
			t.Errorf("CachePath(%q): expected error", uri)
		}
	}
}

func TestBaseName(t *testing.T) {
	cases := map[string]string{
		"s3-msi:bucket/a/b.parquet":       "b.parquet",
		"https://h/files/x/data.zip?dl=1": "data.zip",
		"s3:bucket/top.csv":               "top.csv",
	}
	for uri, want := range cases {
		if got := BaseName(uri); got != want {
			t.Errorf("BaseName(%q) = %q, want %q", uri, got, want)
		}
	}
}

func TestCacheRoot(t *testing.T) {
	home, _ := os.UserHomeDir()
	t.Setenv("PIPELINK_CACHE", "")
	if got, _ := CacheRoot(""); got != filepath.Join(home, ".cache", "pipelink") {
		t.Errorf("default cache root = %q", got)
	}
	t.Setenv("PIPELINK_CACHE", "/env/cache")
	if got, _ := CacheRoot(""); got != "/env/cache" {
		t.Errorf("env cache root = %q", got)
	}
	if got, _ := CacheRoot("~/data/cache"); got != filepath.Join(home, "data", "cache") {
		t.Errorf("entry cache root = %q", got)
	}
}

func TestStamp(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.bin")
	os.WriteFile(p, []byte("hello"), 0644)
	sum := hexsum([]byte("hello"))

	if _, ok := FreshStamp(p); ok {
		t.Fatal("no stamp yet, expected not fresh")
	}
	if err := WriteStamp(p, sum); err != nil {
		t.Fatal(err)
	}
	got, ok := FreshStamp(p)
	if !ok || got != sum {
		t.Fatalf("FreshStamp = %q, %v", got, ok)
	}
	// modifying the file invalidates the stamp
	future := time.Now().Add(time.Hour)
	os.WriteFile(p, []byte("hellO"), 0644)
	os.Chtimes(p, future, future)
	if _, ok := FreshStamp(p); ok {
		t.Fatal("expected stale stamp after modification")
	}
}

func servePayload(t *testing.T, payload []byte) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pkg/data.bin" {
			http.NotFound(w, r)
			return
		}
		w.Write(payload)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestPull_HTTP(t *testing.T) {
	payload := []byte("replication package bytes")
	srv := servePayload(t, payload)
	dst := filepath.Join(t.TempDir(), "c", "data.bin")
	uri := srv.URL + "/pkg/data.bin"

	res, err := Pull(uri, hexsum(payload), dst, false)
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if res.Status != Downloaded || res.SHA256 != hexsum(payload) {
		t.Fatalf("unexpected result %+v", res)
	}
	if b, _ := os.ReadFile(dst); string(b) != string(payload) {
		t.Fatalf("cached content = %q", b)
	}

	// second pull is a no-op
	res, err = Pull(uri, hexsum(payload), dst, false)
	if err != nil || res.Status != Cached {
		t.Fatalf("second pull: %+v, %v", res, err)
	}

	// corrupt the cache (same size, so only --verify or a stale stamp catches it)
	os.WriteFile(dst, []byte("REPLICATION package bytes"), 0644)
	future := time.Now().Add(time.Hour)
	os.Chtimes(dst, future, future)
	res, err = Pull(uri, hexsum(payload), dst, false)
	if err != nil || res.Status != Downloaded {
		t.Fatalf("pull after corruption: %+v, %v", res, err)
	}
	if b, _ := os.ReadFile(dst); string(b) != string(payload) {
		t.Fatalf("cache not repaired: %q", b)
	}
}

func TestPull_HTTP_Verify(t *testing.T) {
	payload := []byte("abc")
	srv := servePayload(t, payload)
	dst := filepath.Join(t.TempDir(), "data.bin")
	uri := srv.URL + "/pkg/data.bin"
	if _, err := Pull(uri, hexsum(payload), dst, false); err != nil {
		t.Fatal(err)
	}
	// corrupt without touching size or mtime: stamp still looks fresh
	st, _ := os.Stat(dst)
	os.WriteFile(dst, []byte("abd"), 0644)
	os.Chtimes(dst, st.ModTime(), st.ModTime())

	if res, _ := Pull(uri, hexsum(payload), dst, false); res.Status != Cached {
		t.Fatalf("without verify the stamp is trusted, got %+v", res)
	}
	res, err := Pull(uri, hexsum(payload), dst, true)
	if err != nil || res.Status != Downloaded {
		t.Fatalf("verify should re-download: %+v, %v", res, err)
	}
}

func TestPull_HTTP_Mismatch(t *testing.T) {
	srv := servePayload(t, []byte("actual"))
	dst := filepath.Join(t.TempDir(), "data.bin")
	_, err := Pull(srv.URL+"/pkg/data.bin", hexsum([]byte("expected")), dst, false)
	if err == nil {
		t.Fatal("expected sha256 mismatch error")
	}
	if _, statErr := os.Stat(dst); !os.IsNotExist(statErr) {
		t.Fatal("bad download must not be left in the cache")
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(dst), "*")); len(left) != 0 {
		t.Fatalf("temp files left behind: %v", left)
	}
}

func TestPull_HTTP_NotFound(t *testing.T) {
	srv := servePayload(t, []byte("x"))
	dst := filepath.Join(t.TempDir(), "data.bin")
	if _, err := Pull(srv.URL+"/missing", "", dst, false); err == nil {
		t.Fatal("expected error on 404")
	}
}

func TestPull_Unpinned(t *testing.T) {
	payload := []byte("no hash given")
	srv := servePayload(t, payload)
	dst := filepath.Join(t.TempDir(), "data.bin")
	res, err := Pull(srv.URL+"/pkg/data.bin", "", dst, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.SHA256 != hexsum(payload) || !res.Unpinned {
		t.Fatalf("unexpected result %+v", res)
	}
}

func requireRclone(t *testing.T) {
	if _, err := exec.LookPath(rcloneBin()); err != nil {
		t.Skip("rclone not on PATH")
	}
}

func TestPull_Rclone(t *testing.T) {
	requireRclone(t)
	srcDir := t.TempDir()
	payload := []byte("13F panel stand-in")
	os.WriteFile(filepath.Join(srcDir, "holdings.parquet"), payload, 0644)
	uri := ":local:" + filepath.Join(srcDir, "holdings.parquet")
	dst := filepath.Join(t.TempDir(), "holdings.parquet")

	res, err := Pull(uri, hexsum(payload), dst, false)
	if err != nil || res.Status != Downloaded {
		t.Fatalf("Pull: %+v, %v", res, err)
	}
	if b, _ := os.ReadFile(dst); string(b) != string(payload) {
		t.Fatalf("content = %q", b)
	}
}

func TestPush_Rclone(t *testing.T) {
	requireRclone(t)
	local := filepath.Join(t.TempDir(), "panel.parquet")
	payload := []byte("to upload")
	os.WriteFile(local, payload, 0644)
	remoteDir := t.TempDir()
	uri := ":local:" + filepath.Join(remoteDir, "sub", "panel.parquet")

	sum, err := Push(local, uri)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if sum != hexsum(payload) {
		t.Fatalf("sha256 = %s", sum)
	}
	if b, _ := os.ReadFile(filepath.Join(remoteDir, "sub", "panel.parquet")); string(b) != string(payload) {
		t.Fatalf("remote content = %q", b)
	}
}

func TestPush_RejectsHTTP(t *testing.T) {
	local := filepath.Join(t.TempDir(), "f")
	os.WriteFile(local, []byte("x"), 0644)
	if _, err := Push(local, "https://example.org/f"); err == nil {
		t.Fatal("expected error pushing to https")
	}
}

func TestPull_Rclone_MissingObject(t *testing.T) {
	requireRclone(t)
	uri := ":local:" + filepath.Join(t.TempDir(), "does-not-exist.parquet")
	dst := filepath.Join(t.TempDir(), "x.parquet")
	_, err := Pull(uri, "", dst, false)
	if err == nil || !strings.Contains(err.Error(), "not found on remote") {
		t.Fatalf("expected a clear not-found error, got %v", err)
	}
}
