package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
)

const (
	apiBase = "https://api.github.com"
	repo    = "asertym/lvm"
)

// ReleaseAsset holds metadata for a lvm binary release asset.
type ReleaseAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// ReleaseWithAssets represents a GitHub release with its downloadable assets.
type ReleaseWithAssets struct {
	TagName     string         `json:"tag_name"`
	PublishedAt string         `json:"published_at"`
	Assets      []ReleaseAsset `json:"assets"`
}

// Release represents a single GitHub release (TagName only).
type Release struct {
	TagName string `json:"tag_name"`
}

// LatestRelease fetches the latest release tag from the asertym/lvm repo.
// Deprecated: use LatestReleaseWithAssets instead.

// LatestReleaseWithAssets fetches the latest release with asset URLs.
func LatestReleaseWithAssets() (*ReleaseWithAssets, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", apiBase, repo)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GitHub API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var r ReleaseWithAssets
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("failed to decode GitHub response: %w", err)
	}
	return &r, nil
}

// AssetForPlatform returns the asset matching the current platform (os/arch).
// Naming convention: lvm_<version>_<os>_<arch> (e.g. lvm_0.1.3_linux_x64)
// Windows assets have a .exe suffix.
func AssetForPlatform(r *ReleaseWithAssets) (*ReleaseAsset, error) {
	osStr := runtime.GOOS
	archStr := runtime.GOARCH
	if archStr == "amd64" {
		archStr = "x64"
	}
	suffix := fmt.Sprintf("%s_%s", osStr, archStr)

	// Exact match first
	for _, a := range r.Assets {
		if strings.Contains(a.Name, suffix) {
			return &a, nil
		}
	}

	return nil, fmt.Errorf("no release asset found for %s/%s in %d assets", osStr, archStr, len(r.Assets))
}

func LatestRelease() (string, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", apiBase, repo)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("GitHub API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var r Release
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", fmt.Errorf("failed to decode GitHub response: %w", err)
	}
	return r.TagName, nil
}

// SemverLess returns true if version a < version b.
// Handles optional leading "v" prefix and semver (major.minor.patch) comparison.
// Only compares major.minor if patch is absent in one version.
func SemverLess(a, b string) bool {
	a = strings.TrimPrefix(a, "v")
	b = strings.TrimPrefix(b, "v")

	// Split into parts.
	av := strings.Split(a, ".")
	bv := strings.Split(b, ".")

	// Ensure both have at least 3 parts.
	for len(av) < 3 {
		av = append(av, "0")
	}
	for len(bv) < 3 {
		bv = append(bv, "0")
	}

	for i := 0; i < 3; i++ {
		ai := intPart(av[i])
		bi := intPart(bv[i])
		if ai < bi {
			return true
		}
		if ai > bi {
			return false
		}
	}
	return false
}

// IntPart returns the integer value of the first sequence of digits in s.
func intPart(s string) int {
	result := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		} else {
			break
		}
	}
	return result
}

// NewerAvailable checks if a version newer than current is available on GitHub.
func NewerAvailable(current string) (bool, string, error) {
	latest, err := LatestRelease()
	if err != nil {
		return false, "", err
	}
	return SemverLess(current, latest), latest, nil
}
