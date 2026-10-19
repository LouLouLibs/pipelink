package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/louloulibs/pipelink/internal/config"
	"github.com/louloulibs/pipelink/internal/display"
	"github.com/louloulibs/pipelink/internal/remote"
	"github.com/spf13/cobra"
)

var verifyHashes bool

var pullCmd = &cobra.Command{
	Use:   "pull <config.toml>",
	Short: "Download remote entries into the local cache and verify them",
	Long: `For every entry of type "remote", download source.uri (an rclone remote:path or
an http(s) URL) into the cache and check it against source.sha256. Files already
cached with a matching hash are skipped. Run "pipelink link" afterwards to create
the symlinks.

The cache is source.cache if set, else $PIPELINK_CACHE, else ~/.cache/pipelink.`,
	Args: cobra.ExactArgs(1),
	RunE: runPull,
}

var pushCmd = &cobra.Command{
	Use:   "push <local-file> <remote:path>",
	Short: "Upload a file with rclone and print its sha256 and a config snippet",
	Long: `Upload a local file to an rclone remote:path, then print its sha256 together with
TOML and Nickel snippets for a "remote" entry. Config files are not modified.`,
	Args: cobra.ExactArgs(2),
	RunE: runPush,
}

func init() {
	pullCmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "list what would be downloaded without fetching")
	pullCmd.Flags().BoolVar(&verifyHashes, "verify", false, "re-hash cached files instead of trusting their stamps")
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(pushCmd)
}

// remoteCachePath returns where a remote entry's file lives in the local cache.
func remoteCachePath(e config.Entry) (string, error) {
	if e.Source.URI == "" {
		return "", fmt.Errorf("remote entry has no source.uri")
	}
	root, err := remote.CacheRoot(e.Source.Cache)
	if err != nil {
		return "", err
	}
	return remote.CachePath(root, e.Source.URI)
}

// resolveRemote rewrites a remote entry as a single-file entry whose source is
// the cached copy, so link and validate treat it like any local file.
func resolveRemote(e config.Entry) (config.Entry, error) {
	p, err := remoteCachePath(e)
	if err != nil {
		return e, err
	}
	out := e
	out.Metadata.Type = "file"
	out.Source = config.Source{Directory: filepath.Dir(p), File: config.Strings(filepath.Base(p))}
	return out, nil
}

func sortedNames(entries map[string]config.Entry) []string {
	names := make([]string, 0, len(entries))
	for n := range entries {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func runPull(cmd *cobra.Command, args []string) error {
	entries, err := config.LoadConfig(args[0])
	if err != nil {
		display.ErrorMsg(fmt.Sprintf("Failed to load config: %v", err))
		return err
	}

	var failed int
	n := 0
	for _, name := range sortedNames(entries) {
		e := entries[name]
		if e.Metadata.Type != "remote" {
			continue
		}
		n++
		dst, err := remoteCachePath(e)
		if err != nil {
			display.PullLine(name, "error", e.Source.URI, "")
			display.ErrorMsg(err.Error())
			failed++
			continue
		}
		if dryRun {
			state := "missing"
			if _, err := os.Stat(dst); err == nil {
				state = "present"
			}
			display.PullLine(name, state, e.Source.URI, dst)
			continue
		}
		res, err := remote.Pull(e.Source.URI, e.Source.SHA256, dst, verifyHashes)
		if err != nil {
			display.PullLine(name, "error", e.Source.URI, dst)
			display.ErrorMsg(err.Error())
			failed++
			continue
		}
		display.PullLine(name, string(res.Status), e.Source.URI, dst)
		if res.Unpinned {
			display.VerboseMsg(fmt.Sprintf("no sha256 pinned for %s; current content is sha256 = %q", name, res.SHA256))
		}
	}

	if n == 0 {
		fmt.Fprintln(os.Stderr, "no remote entries in config")
	}
	if failed > 0 {
		return fmt.Errorf("%d remote entr(ies) failed", failed)
	}
	return nil
}

func runPush(cmd *cobra.Command, args []string) error {
	local, uri := args[0], args[1]
	sum, err := remote.Push(local, uri)
	if err != nil {
		display.ErrorMsg(err.Error())
		return err
	}
	fmt.Fprintf(os.Stderr, "uploaded %s -> %s\n\n", local, uri)
	fmt.Printf("sha256 = %q\n\n", sum)
	fmt.Printf("# TOML\n[NAME.metadata]\ntype = \"remote\"\n\n[NAME.source]\nuri = %q\nsha256 = %q\n\n[NAME.target]\ndirectory = \"input\"\n\n", uri, sum)
	fmt.Printf("# Nickel\nNAME | Link = 'remote {\n  source = { uri = %q, sha256 = %q },\n  target = { directory = \"input\" },\n},\n", uri, sum)
	return nil
}
