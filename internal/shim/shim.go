package shim

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
)

// KnownBinaries is the canonical set of llama.cpp command names.
var KnownBinaries = []string{
	"llama-cli",
	"llama-server",
	"llama-bench",
	"llama-quantize",
	"llama-embedding",
	"llama-perplexity",
	"llama-tokenize",
	"llama-run",
	"llama-simple",
	"llama-imatrix",
}

// Manager handles shim creation and updates.
type Manager struct {
	shimsDir string
	lvmHome  string
}

// NewManager creates a shim manager.
func NewManager(shimsDir, lvmHome string) *Manager {
	return &Manager{
		shimsDir: shimsDir,
		lvmHome:  lvmHome,
	}
}

// EnsureAll creates shims for all known binaries if they don't exist yet.
func (m *Manager) EnsureAll() error {
	if err := os.MkdirAll(m.shimsDir, 0755); err != nil {
		return fmt.Errorf("cannot create shims dir: %w", err)
	}
	for _, name := range KnownBinaries {
		if err := m.Ensure(name); err != nil {
			return err
		}
	}
	return nil
}

// Ensure creates a shim for a single binary name if it doesn't already exist.
func (m *Manager) Ensure(binaryName string) error {
	shimPath := m.ShimPath(binaryName)
	if _, err := os.Stat(shimPath); err == nil {
		return nil
	}
	return m.Create(binaryName)
}

// Create (re)creates a shim for a given binary name.
func (m *Manager) Create(binaryName string) error {
	if err := os.MkdirAll(m.shimsDir, 0755); err != nil {
		return err
	}
	shimPath := m.ShimPath(binaryName)

	if runtime.GOOS == "windows" {
		return m.createWindowsShim(shimPath, binaryName)
	}
	return m.createUnixShim(shimPath, binaryName)
}

// ShimPath returns the full path of the shim file for a binary name.
func (m *Manager) ShimPath(binaryName string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(m.shimsDir, binaryName+".cmd")
	}
	return filepath.Join(m.shimsDir, binaryName)
}

// createUnixShim writes a POSIX shell shim script.
func (m *Manager) createUnixShim(shimPath, binaryName string) error {
	const tmpl = `#!/bin/sh
# lvm shim — {{.BinaryName}}
LVM_HOME="{{.LvmHome}}"
ACTIVE_FILE="$LVM_HOME/active"

if [ ! -f "$ACTIVE_FILE" ]; then
	echo "lvm: no active version set. Run: lvm use <version>" >&2
	exit 1
fi

VERSION=$(cat "$ACTIVE_FILE")
BINARY="$LVM_HOME/versions/$VERSION/{{.BinaryName}}"

# Fallback to legacy main binary if modern name doesn't exist
if [ ! -f "$BINARY" ]; then
	BINARY="$LVM_HOME/versions/$VERSION/main"
fi

if [ ! -f "$BINARY" ]; then
	echo "lvm: binary '{{.BinaryName}}' not found in version $VERSION" >&2
	exit 1
fi

exec "$BINARY" "$@"
`
	data := struct {
		BinaryName string
		LvmHome    string
	}{binaryName, m.lvmHome}

	t, err := template.New("unix_shim").Parse(tmpl)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(shimPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("cannot create shim at %s: %w", shimPath, err)
	}
	defer f.Close()

	return t.Execute(f, data)
}

// createWindowsShim writes a .cmd batch file shim.
func (m *Manager) createWindowsShim(shimPath, binaryName string) error {
	const cmdTmpl = `@echo off
setlocal enabledelayedexpansion
rem lvm shim — {{.BinaryName}}
set "LVM_HOME={{.LvmHome}}"
set "ACTIVE_FILE=%LVM_HOME%\active"

if not exist "%ACTIVE_FILE%" (
	echo lvm: no active version set. Run: lvm use ^<version^> 1>&2
	exit /b 1
)

set /p VERSION=<"%ACTIVE_FILE%"
set "VERSION_DIR=%LVM_HOME%\versions\!VERSION!"
set "BINARY=!VERSION_DIR!\{{.BinaryName}}.exe"

rem Fallback to legacy main.exe if modern name doesn't exist
if not exist "!BINARY!" (
	set "BINARY=!VERSION_DIR!\main.exe"
)

if not exist "!BINARY!" (
	echo lvm: binary '{{.BinaryName}}' not found in version !VERSION! 1>&2
	exit /b 1
)

"!BINARY!" %*
`
	data := struct {
		BinaryName string
		LvmHome    string
	}{binaryName, m.lvmHome}

	t, err := template.New("win_shim").Parse(cmdTmpl)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(shimPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("cannot create shim at %s: %w", shimPath, err)
	}
	defer f.Close()

	return t.Execute(f, data)
}

// List returns all shim names currently installed.
func (m *Manager) List() ([]string, error) {
	entries, err := os.ReadDir(m.shimsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		name := e.Name()
		name = strings.TrimSuffix(name, ".cmd")
		names = append(names, name)
	}
	return names, nil
}