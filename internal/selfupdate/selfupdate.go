package selfupdate

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Update downloads the latest lvm binary and replaces the running executable.
// Returns nil on success. On Windows it schedules the replacement for next start.
func Update(downloadedURL string, binPath string) error {
	if binPath == "" {
		var err error
		binPath, err = os.Executable()
		if err != nil {
			return fmt.Errorf("cannot determine binary path: %w", err)
		}
	}

	dir := filepath.Dir(binPath)
	base := filepath.Base(binPath)

	switch runtime.GOOS {
	case "windows":
		return updateWindows(downloadedURL, dir, base)
	default:
		return updateUnix(downloadedURL, dir, binPath)
	}
}

func updateUnix(url, dir, binPath string) error {
	tmpFile := filepath.Join(dir, ".lvm-update-tmp")
	if err := download(url, tmpFile); err != nil {
		return err
	}
	defer os.Remove(tmpFile)

	if err := os.Chmod(tmpFile, 0755); err != nil {
		return fmt.Errorf("chmod failed: %w", err)
	}

	if err := os.Rename(tmpFile, binPath); err != nil {
		return fmt.Errorf("replace binary failed (you may need to run with elevated privileges): %w", err)
	}
	return nil
}

func updateWindows(url, dir, base string) error {
	newName := ".lvm-new.exe"
	tmpFile := filepath.Join(dir, newName)
	if err := download(url, tmpFile); err != nil {
		return err
	}

	binDir := filepath.Dir(tmpFile)

	// Build a batch script that replaces the binary and restarts lvm.
	scriptName := ".lvm-update-helper.bat"
	scriptPath := filepath.Join(dir, scriptName)

	script := fmt.Sprintf(`@echo off
timeout /t 1 /nobreak >nul
move /y "%s\%s" "%s\%s"
del /f "%s"
start "" "%s\%s"
`, binDir, newName, binDir, base, filepath.Join(binDir, newName), binDir, base)

	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return fmt.Errorf("cannot write helper script: %w", err)
	}

	// Launch the batch file and exit so the current binary is no longer locked.
	cmd := exec.Command(scriptPath)
	cmd.Dir = dir
	_ = cmd.Start()

	// Remove the downloaded exe now since the script will move it.
	os.Remove(tmpFile)

	return nil
}

func download(url, destPath string) error {
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
		return fmt.Errorf("cannot create temp file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write failed: %w", err)
	}
	return nil
}
