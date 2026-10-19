// Package remote fetches pinned files from rclone remotes or HTTP(S) URLs into
// a local cache, so that remote inputs can be symlinked like local ones.
//
// URIs are either "https://host/path" (fetched with net/http) or an rclone path
// "remote:path" (fetched by shelling out to rclone). The cache mirrors the URI:
// <root>/<remote>/<path> or <root>/<host>/<path>.
package remote

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// Status says what Pull did.
type Status string

const (
	Cached     Status = "cached"
	Downloaded Status = "downloaded"
)

// Result describes the outcome of a Pull.
type Result struct {
	Status   Status
	SHA256   string
	Unpinned bool // no sha256 was given, so the version is not pinned
}

// IsHTTP reports whether uri is fetched over HTTP(S) rather than with rclone.
func IsHTTP(uri string) bool {
	return strings.HasPrefix(uri, "https://") || strings.HasPrefix(uri, "http://")
}

// CacheRoot returns the cache directory: the entry's own setting if given,
// else $PIPELINK_CACHE, else ~/.cache/pipelink. A leading ~ is expanded.
func CacheRoot(entryCache string) (string, error) {
	root := entryCache
	if root == "" {
		root = os.Getenv("PIPELINK_CACHE")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}
	if root == "" {
		return filepath.Join(home, ".cache", "pipelink"), nil
	}
	if root == "~" || strings.HasPrefix(root, "~/") {
		root = filepath.Join(home, root[1:])
	}
	return root, nil
}

// splitURI returns the cache namespace (rclone remote name or host) and the path.
func splitURI(uri string) (string, string, error) {
	if IsHTTP(uri) {
		u, err := url.Parse(uri)
		if err != nil {
			return "", "", fmt.Errorf("parsing %q: %w", uri, err)
		}
		return strings.ReplaceAll(u.Host, ":", "_"), u.Path, nil
	}
	// rclone: "remote:path", or an on-the-fly backend ":local:path"
	rest, prefix := uri, ""
	if strings.HasPrefix(uri, ":") {
		rest, prefix = uri[1:], "_"
	}
	i := strings.Index(rest, ":")
	if i <= 0 {
		return "", "", fmt.Errorf("%q is neither an http(s) URL nor an rclone remote:path", uri)
	}
	return prefix + rest[:i], rest[i+1:], nil
}

// CachePath maps uri to its location under root. It rejects URIs whose path is
// empty, names a directory, or contains ".." (which could escape the cache).
func CachePath(root, uri string) (string, error) {
	ns, p, err := splitURI(uri)
	if err != nil {
		return "", err
	}
	if p == "" || strings.HasSuffix(p, "/") {
		return "", fmt.Errorf("%q does not name a file", uri)
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return "", fmt.Errorf("%q contains '..'", uri)
		}
	}
	return filepath.Join(root, ns, filepath.FromSlash(strings.TrimLeft(p, "/"))), nil
}

// BaseName is the file name at the end of the URI's path (query string dropped).
func BaseName(uri string) string {
	_, p, err := splitURI(uri)
	if err != nil {
		return ""
	}
	return path.Base(p)
}

// HashFile returns the hex sha256 of a file.
func HashFile(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// stampPath is the sidecar recording a verified hash: ".<name>.pipelink".
func stampPath(p string) string {
	return filepath.Join(filepath.Dir(p), "."+filepath.Base(p)+".pipelink")
}

// WriteStamp records that p (at its current size and mtime) hashes to sum,
// so later pulls can skip re-hashing large files.
func WriteStamp(p, sum string) error {
	st, err := os.Stat(p)
	if err != nil {
		return err
	}
	line := fmt.Sprintf("%s %d %d\n", sum, st.Size(), st.ModTime().UnixNano())
	return os.WriteFile(stampPath(p), []byte(line), 0644)
}

// FreshStamp returns the recorded hash if the stamp exists and the file's size
// and mtime still match it.
func FreshStamp(p string) (string, bool) {
	b, err := os.ReadFile(stampPath(p))
	if err != nil {
		return "", false
	}
	f := strings.Fields(string(b))
	st, err := os.Stat(p)
	if len(f) != 3 || err != nil {
		return "", false
	}
	size, err1 := strconv.ParseInt(f[1], 10, 64)
	mtime, err2 := strconv.ParseInt(f[2], 10, 64)
	if err1 != nil || err2 != nil || size != st.Size() || mtime != st.ModTime().UnixNano() {
		return "", false
	}
	return f[0], true
}

func rcloneBin() string {
	if b := os.Getenv("PIPELINK_RCLONE"); b != "" {
		return b
	}
	return "rclone"
}

func runRclone(args ...string) error {
	bin := rcloneBin()
	if _, err := exec.LookPath(bin); err != nil {
		return fmt.Errorf("rclone not found (install it or set PIPELINK_RCLONE): %w", err)
	}
	out, err := exec.Command(bin, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("rclone %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return nil
}

func fetchHTTP(uri, dst string) error {
	resp, err := http.Get(uri)
	if err != nil {
		return fmt.Errorf("GET %s: %w", uri, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", uri, resp.Status)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return fmt.Errorf("downloading %s: %w", uri, err)
	}
	return f.Close()
}

// cachedHash returns the hash of an existing cached file, trusting a fresh
// stamp unless verify is set. ok is false if dst does not exist.
func cachedHash(dst string, verify bool) (sum string, ok bool, err error) {
	if _, err := os.Stat(dst); errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if !verify {
		if s, fresh := FreshStamp(dst); fresh {
			return s, true, nil
		}
	}
	sum, err = HashFile(dst)
	if err != nil {
		return "", false, err
	}
	return sum, true, WriteStamp(dst, sum)
}

// Pull makes dst a verified copy of uri. If want is non-empty the content must
// hash to it; a cached copy that matches is kept, anything else is re-fetched.
// Downloads go to a temporary file next to dst and are renamed into place only
// after verification, so a failed or mismatched download never leaves a file
// at dst.
func Pull(uri, want, dst string, verify bool) (Result, error) {
	sum, ok, err := cachedHash(dst, verify)
	if err != nil {
		return Result{}, err
	}
	if ok && (want == "" || sum == want) {
		return Result{Status: Cached, SHA256: sum, Unpinned: want == ""}, nil
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return Result{}, err
	}
	tmpf, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".partial-*")
	if err != nil {
		return Result{}, err
	}
	tmp := tmpf.Name()
	tmpf.Close()
	defer os.Remove(tmp) // no-op once renamed

	if IsHTTP(uri) {
		err = fetchHTTP(uri, tmp)
	} else {
		os.Remove(tmp) // let rclone create it, so it never compares against an empty file
		err = runRclone("copyto", uri, tmp)
		_, statErr := os.Stat(tmp)
		// A missing source surfaces differently by backend: S3 exits 0 and
		// copies nothing, the local backend exits 3 with "not found".
		if (err == nil && statErr != nil) || (err != nil && strings.Contains(err.Error(), "not found")) {
			err = fmt.Errorf("%s not found on remote", uri)
		}
	}
	if err != nil {
		return Result{}, err
	}

	got, err := HashFile(tmp)
	if err != nil {
		return Result{}, err
	}
	if want != "" && got != want {
		return Result{}, fmt.Errorf("sha256 mismatch for %s: want %s, got %s", uri, want, got)
	}
	if err := os.Rename(tmp, dst); err != nil {
		return Result{}, err
	}
	if err := WriteStamp(dst, got); err != nil {
		return Result{}, err
	}
	return Result{Status: Downloaded, SHA256: got, Unpinned: want == ""}, nil
}

// Push uploads local to an rclone uri and returns its sha256.
func Push(local, uri string) (string, error) {
	if IsHTTP(uri) {
		return "", fmt.Errorf("cannot push to %s: push needs an rclone remote:path", uri)
	}
	if _, _, err := splitURI(uri); err != nil {
		return "", err
	}
	sum, err := HashFile(local)
	if err != nil {
		return "", err
	}
	if err := runRclone("copyto", local, uri); err != nil {
		return "", err
	}
	return sum, nil
}
