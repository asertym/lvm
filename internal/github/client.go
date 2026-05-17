package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	apiBase      = "https://api.github.com"
	repo         = "ggerganov/llama.cpp"
	cacheTimeout = time.Hour
)

// Release represents a single GitHub release.
type Release struct {
	TagName    string  `json:"tag_name"`
	Name       string  `json:"name"`
	PreRelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
	PublishedAt string `json:"published_at"`
	HTMLURL    string  `json:"html_url"`
}

// Asset represents a downloadable file attached to a release.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// Client fetches release data from the GitHub API with local caching.
type Client struct {
	cacheDir string
	http     *http.Client
}

// NewClient creates a GitHub API client. cacheDir is where release lists are cached.
func NewClient(cacheDir string) *Client {
	return &Client{
		cacheDir: cacheDir,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

// ListReleases returns all releases, using cache if fresh enough.
func (c *Client) ListReleases() ([]Release, error) {
	cached, err := c.loadCache()
	if err == nil {
		return cached, nil
	}

	releases, err := c.fetchReleases()
	if err != nil {
		return nil, err
	}

	_ = c.saveCache(releases) // best-effort
	return releases, nil
}

// LatestStable returns the newest non-prerelease release.
func (c *Client) LatestStable() (*Release, error) {
	releases, err := c.ListReleases()
	if err != nil {
		return nil, err
	}
	for _, r := range releases {
		if !r.PreRelease {
			return &r, nil
		}
	}
	return nil, fmt.Errorf("no stable release found")
}

// LatestBeta returns the newest release including pre-releases.
func (c *Client) LatestBeta() (*Release, error) {
	releases, err := c.ListReleases()
	if err != nil {
		return nil, err
	}
	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found")
	}
	return &releases[0], nil
}

// FindRelease finds a release by exact tag name (e.g. "b3412").
func (c *Client) FindRelease(tag string) (*Release, error) {
	// Normalize: accept "b3412" or "b3412" with or without the "b" prefix.
	normalized := tag
	if !strings.HasPrefix(normalized, "b") {
		normalized = "b" + normalized
	}

	releases, err := c.ListReleases()
	if err != nil {
		return nil, err
	}
	for _, r := range releases {
		if r.TagName == normalized || r.TagName == tag {
			return &r, nil
		}
	}

	// Not in cache — try direct API call (might be an old release).
	url := fmt.Sprintf("%s/repos/%s/releases/tags/%s", apiBase, repo, tag)
	return c.fetchRelease(url)
}

// FindAsset finds the best matching asset for a given platform suffix.
// It tries an exact suffix match first, then falls back to partial matching.
func (r *Release) FindAsset(suffix string) (*Asset, error) {
	// Prefer .tar.gz on Linux/macOS, .zip on Windows.
	extensions := []string{".tar.gz", ".zip"}

	for _, ext := range extensions {
		for _, asset := range r.Assets {
			name := strings.ToLower(asset.Name)
			if strings.Contains(name, strings.ToLower(suffix)) && strings.HasSuffix(name, ext) {
				return &asset, nil
			}
		}
	}

	// List available assets in the error so the user can pick manually.
	names := make([]string, 0, len(r.Assets))
	for _, a := range r.Assets {
		names = append(names, a.Name)
	}
	return nil, fmt.Errorf(
		"no asset matching %q found in release %s\navailable assets:\n  %s",
		suffix, r.TagName, strings.Join(names, "\n  "),
	)
}

// FindSHASUM returns the checksum asset for a given asset name, if present.
func (r *Release) FindSHASUM(assetName string) *Asset {
	targets := []string{
		assetName + ".sha256",
		"SHA256SUMS",
		"checksums.txt",
	}
	for _, target := range targets {
		for _, a := range r.Assets {
			if strings.EqualFold(a.Name, target) {
				return &a
			}
		}
	}
	return nil
}

// InvalidateCache deletes the local release cache, forcing a fresh fetch next time.
func (c *Client) InvalidateCache() error {
	return os.Remove(c.cachePath())
}

// --- internal ---

func (c *Client) fetchReleases() ([]Release, error) {
	url := fmt.Sprintf("%s/repos/%s/releases?per_page=50", apiBase, repo)
	resp, err := c.http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GitHub API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("failed to decode GitHub response: %w", err)
	}
	return releases, nil
}

func (c *Client) fetchRelease(url string) (*Release, error) {
	resp, err := c.http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GitHub API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("release not found")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode release: %w", err)
	}
	return &release, nil
}

type cacheFile struct {
	FetchedAt time.Time `json:"fetched_at"`
	Releases  []Release `json:"releases"`
}

func (c *Client) cachePath() string {
	return filepath.Join(c.cacheDir, "releases_cache.json")
}

func (c *Client) loadCache() ([]Release, error) {
	f, err := os.Open(c.cachePath())
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cache cacheFile
	if err := json.NewDecoder(f).Decode(&cache); err != nil {
		return nil, err
	}

	if time.Since(cache.FetchedAt) > cacheTimeout {
		return nil, fmt.Errorf("cache expired")
	}

	return cache.Releases, nil
}

func (c *Client) saveCache(releases []Release) error {
	if err := os.MkdirAll(c.cacheDir, 0755); err != nil {
		return err
	}

	f, err := os.Create(c.cachePath())
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(cacheFile{
		FetchedAt: time.Now(),
		Releases:  releases,
	})
}

// DownloadFile downloads a URL to a local path, reporting progress via the callback.
func DownloadFile(url, destPath string, progress func(downloaded, total int64)) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("cannot create file: %w", err)
	}
	defer f.Close()

	total := resp.ContentLength
	reader := &progressReader{r: resp.Body, total: total, callback: progress}
	_, err = io.Copy(f, reader)
	return err
}

type progressReader struct {
	r          io.Reader
	downloaded int64
	total      int64
	callback   func(int64, int64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	pr.downloaded += int64(n)
	if pr.callback != nil {
		pr.callback(pr.downloaded, pr.total)
	}
	return n, err
}
