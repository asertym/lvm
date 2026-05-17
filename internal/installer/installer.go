package installer

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	gh "lvm/internal/github"
)

// Install downloads and extracts a release asset into destDir.
// destDir will be created if it doesn't exist.
func Install(asset *Asset, destDir string, progress func(downloaded, total int64)) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("cannot create version dir: %w", err)
	}

	// Download to a temp file.
	tmpFile := filepath.Join(os.TempDir(), asset.Name)
	defer os.Remove(tmpFile)

	if err := gh.DownloadFile(asset.URL, tmpFile, progress); err != nil {
		return err
	}

	// Verify checksum if provided.
	if asset.SHA256 != "" {
		if err := verifySHA256(tmpFile, asset.SHA256); err != nil {
			return err
		}
	}

	// Extract.
	if strings.HasSuffix(asset.Name, ".tar.gz") || strings.HasSuffix(asset.Name, ".tgz") {
		return extractTarGz(tmpFile, destDir)
	}
	if strings.HasSuffix(asset.Name, ".zip") {
		return extractZip(tmpFile, destDir)
	}

	return fmt.Errorf("unsupported archive format: %s", asset.Name)
}

// Asset holds everything needed to download and verify a release binary.
type Asset struct {
	Name   string
	URL    string
	Size   int64
	SHA256 string // may be empty if not available
}

// verifySHA256 checks that the file at path matches the expected hex digest.
func verifySHA256(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, expected) {
		return fmt.Errorf("SHA256 mismatch\n  expected: %s\n  got:      %s", expected, got)
	}
	return nil
}

// extractTarGz extracts a .tar.gz archive into destDir, stripping the top-level directory.
func extractTarGz(src, destDir string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("not a valid gzip file: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read error: %w", err)
		}

		// Strip the top-level directory from the path.
		rel := stripTopDir(hdr.Name)
		if rel == "" {
			continue
		}

		target := filepath.Join(destDir, rel)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			if err := writeFile(target, tr, hdr.FileInfo().Mode()); err != nil {
				return err
			}
		}
	}
	return nil
}

// extractZip extracts a .zip archive into destDir, stripping the top-level directory.
func extractZip(src, destDir string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("not a valid zip file: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		rel := stripTopDir(f.Name)
		if rel == "" {
			continue
		}

		target := filepath.Join(destDir, rel)

		if f.FileInfo().IsDir() {
			os.MkdirAll(target, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		mode := f.FileInfo().Mode()
		if err := writeFile(target, rc, mode); err != nil {
			rc.Close()
			return err
		}
		rc.Close()
	}
	return nil
}

// writeFile writes reader content to path with the given file mode.
func writeFile(path string, r io.Reader, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode|0111)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

// stripTopDir removes the first path component from a tar/zip entry name.
// "llama-b3412-bin-ubuntu-x64/llama-cli" → "llama-cli"
// Already-flat paths are returned as-is.
func stripTopDir(name string) string {
	name = filepath.ToSlash(name)
	parts := strings.SplitN(name, "/", 2)
	if len(parts) < 2 {
		return name
	}
	return parts[1]
}
