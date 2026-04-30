package commands

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Self-update via GitHub Releases.
//
// Flow:
//  1. GET https://api.github.com/repos/<owner>/<repo>/releases/latest
//  2. Pick the asset matching `rh_v<tag>_<os>_<arch>.tar.gz` and `checksums.txt`
//  3. Download archive, verify SHA-256 against checksums.txt
//  4. Extract `rh` binary from the tar.gz
//  5. Atomically swap the running binary on disk
//     (rename current → ${exe}.old, rename temp → exe)
//
// Why not pull in a self-update library? The whole flow is ~200 LOC of
// stdlib, the dependency would dwarf the feature, and we control both
// release packaging (.goreleaser.yaml) and consumer (this command) so
// the integration is trivial. macOS + Linux can both rename over a
// running binary; we don't ship Windows so its peculiarities don't
// matter here.

const (
	updateOwner = "JackZhao98"
	updateRepo  = "robinhood-cli"
	updateBin   = "rh"

	updateAPITimeout      = 15 * time.Second
	updateDownloadTimeout = 120 * time.Second
)

func newUpdateCmd() *cobra.Command {
	var checkOnly bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Self-update rh from the latest GitHub release",
		Long: `Check GitHub for the latest tagged release of robinhood-cli and (unless
--check is given) download + replace the running binary in place.

The current build's version is shown alongside the latest available so
you can decide whether the update is worth applying. The previous
binary is preserved as <path>.old in case the update needs to be
reverted manually.

The download is verified against the release's checksums.txt before
the binary is swapped — a missing or mismatched checksum aborts the
update without touching the existing install.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(cmd.Context(), checkOnly)
		},
	}
	cmd.Flags().BoolVar(&checkOnly, "check", false,
		"Only report current vs latest version; do not download or install.")
	return cmd
}

// ----- GitHub API DTOs ---------------------------------------------------

type ghRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	PublishedAt string    `json:"published_at"`
	Prerelease  bool      `json:"prerelease"`
	Assets      []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
	Size        int64  `json:"size"`
}

func fetchLatestRelease(ctx context.Context) (*ghRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", updateOwner, updateRepo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "rh-cli/"+buildVersion)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch release: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, fmt.Errorf("no published releases for %s/%s yet — has the first GitHub Action release run successfully?", updateOwner, updateRepo)
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("github api %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}
	return &rel, nil
}

// pickAssets selects the platform archive + checksums.txt out of release assets.
//
// Asset naming follows the .goreleaser.yaml `archives.name_template` and
// GoReleaser's convention that `.Version` strips the leading "v" from
// the git tag (so tag `v1.2.1` produces archives named `rh_1.2.1_*`).
func pickAssets(rel *ghRelease) (archive *ghAsset, checksums *ghAsset, err error) {
	versionNoV := strings.TrimPrefix(rel.TagName, "v")
	wantArchive := fmt.Sprintf("rh_%s_%s_%s.tar.gz", versionNoV, runtime.GOOS, runtime.GOARCH)
	for i := range rel.Assets {
		a := &rel.Assets[i]
		if a.Name == wantArchive {
			archive = a
		}
		if a.Name == "checksums.txt" {
			checksums = a
		}
	}
	if archive == nil {
		// Build a friendly hint listing what was actually published.
		var got []string
		for _, a := range rel.Assets {
			got = append(got, a.Name)
		}
		return nil, nil, fmt.Errorf("no asset %q in release %s (got: %s)",
			wantArchive, rel.TagName, strings.Join(got, ", "))
	}
	if checksums == nil {
		return nil, nil, fmt.Errorf("no checksums.txt in release %s — refusing to update without verification", rel.TagName)
	}
	return archive, checksums, nil
}

// ----- download / verify / extract / swap --------------------------------

func downloadBytes(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "rh-cli/"+buildVersion)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s returned %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// extractBinary pulls "rh" out of a tar.gz archive's bytes.
func extractBinary(archive []byte) ([]byte, error) {
	gr, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("gunzip: %w", err)
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar: %w", err)
		}
		if filepath.Base(hdr.Name) == updateBin && hdr.Typeflag == tar.TypeReg {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("binary %q not found in archive", updateBin)
}

// parseChecksums returns map[filename]sha256hex from a goreleaser-style
// `<sha256hex>  <filename>` text file.
func parseChecksums(b []byte) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) != 2 {
			continue
		}
		out[fields[1]] = fields[0]
	}
	return out
}

// replaceSelf atomically swaps the running binary at exePath with newBytes.
// On macOS and Linux a running binary's inode can be unlinked while it
// stays mapped; the next exec picks up the new file. We keep the prior
// version at <exePath>.old so the user can revert with one mv.
func replaceSelf(exePath string, newBytes []byte) error {
	dir := filepath.Dir(exePath)
	tmp, err := os.CreateTemp(dir, ".rh.update.*")
	if err != nil {
		return fmt.Errorf("create temp in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := tmp.Write(newBytes); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		cleanup()
		return fmt.Errorf("chmod temp: %w", err)
	}

	backup := exePath + ".old"
	_ = os.Remove(backup) // ok if missing
	if err := os.Rename(exePath, backup); err != nil {
		cleanup()
		return fmt.Errorf("back up current binary: %w", err)
	}
	if err := os.Rename(tmpPath, exePath); err != nil {
		// Try to put the old one back so the user isn't left with a broken install.
		_ = os.Rename(backup, exePath)
		return fmt.Errorf("install new binary: %w", err)
	}
	return nil
}

// ----- top-level orchestration ------------------------------------------

// normalizeTag strips the leading "v" and the goreleaser-style "-dirty"
// suffix. Used for "are we already on this version" comparisons so a
// build stamped `1.2.1` (old goreleaser config without leading "v")
// matches a tag named `v1.2.1` published by the new config.
//
// We deliberately do NOT strip `-N-gSHA` (commits-ahead-of-tag): that
// signals a dev build that is genuinely past the latest release and
// the user may legitimately want to roll forward (or back).
func normalizeTag(s string) string {
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimSuffix(s, "-dirty")
	return s
}

func runUpdate(ctx context.Context, checkOnly bool) error {
	fmt.Printf("current : %s (commit %s, built %s)\n", buildVersion, buildCommit, buildBuiltAt)

	apiCtx, cancel := context.WithTimeout(ctx, updateAPITimeout)
	defer cancel()
	rel, err := fetchLatestRelease(apiCtx)
	if err != nil {
		return err
	}

	suffix := ""
	if rel.Prerelease {
		suffix = " (pre-release)"
	}
	fmt.Printf("latest  : %s%s\n", rel.TagName, suffix)

	if normalizeTag(rel.TagName) == normalizeTag(buildVersion) {
		fmt.Println("✓ already on the latest version")
		return nil
	}

	if checkOnly {
		fmt.Println()
		fmt.Println("→ run `rh update` to install")
		return nil
	}

	archive, checksumAsset, err := pickAssets(rel)
	if err != nil {
		return err
	}
	fmt.Printf("→ downloading %s (%d bytes)\n", archive.Name, archive.Size)

	dlCtx, cancelDL := context.WithTimeout(ctx, updateDownloadTimeout)
	defer cancelDL()

	archiveBytes, err := downloadBytes(dlCtx, archive.DownloadURL)
	if err != nil {
		return err
	}
	checksumsBytes, err := downloadBytes(dlCtx, checksumAsset.DownloadURL)
	if err != nil {
		return err
	}

	fmt.Println("→ verifying checksum")
	checksums := parseChecksums(checksumsBytes)
	expectedHex, ok := checksums[archive.Name]
	if !ok {
		return fmt.Errorf("checksums.txt has no entry for %s — refusing to update", archive.Name)
	}
	sum := sha256.Sum256(archiveBytes)
	actualHex := hex.EncodeToString(sum[:])
	if !strings.EqualFold(expectedHex, actualHex) {
		return fmt.Errorf("checksum mismatch: expected %s, got %s — release may be corrupt", expectedHex, actualHex)
	}

	fmt.Println("→ extracting binary")
	binBytes, err := extractBinary(archiveBytes)
	if err != nil {
		return err
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate self: %w", err)
	}
	// Resolve symlinks so we don't accidentally rewrite a different file.
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	fmt.Printf("→ swapping %s\n", exe)
	if err := replaceSelf(exe, binBytes); err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("ok updated %s → %s\n", buildVersion, rel.TagName)
	fmt.Printf("(previous binary preserved at %s.old)\n", exe)
	return nil
}
