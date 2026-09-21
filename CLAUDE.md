# llava — llama.cpp Version Manager

## Project Overview

**llava** is a Go-based CLI version manager for [llama.cpp](https://github.com/ggerganov/llama.cpp) builds. It lets users install, switch between, and update multiple llama.cpp versions across stable and beta channels. It works on **Linux**, **macOS**, and **Windows**.

- **Language**: Go 1.23+ (module `llava`)
- **CLI framework**: [Cobra](https://github.com/spf13/cobra)
- **Interactive UI**: [Huh](https://github.com/charmbracelet/huh) (arrow-key selections)
- **Current version**: 0.2.4 (declared as `var version = "0.2.4"` in `main.go`)
- **GitHub repo**: `asertym/lvm`

---

## Quick Start / Setup

### Build from source

```bash
make build        # cross-compile for current platform
make build-windows # Windows
make build-linux   # Linux
make build-macos   # macOS
```

### Install via script

- **Linux/macOS**: `curl -sSL https://github.com/asertym/lvm/releases/latest/download/install.sh | sh`
- **Windows (PowerShell)**: Download and run `install.ps1` from releases

### First-run workflow

```bash
llava init       # Sets up shims, adds shims dir to PATH (auto-detected shell profiles)
llava install latest   # Downloads and installs latest stable llama.cpp build
```

---

## Core Architecture

```
lvm/
├── main.go                     # Entry point: llavaHome(), root command, cmdVersion(), cmdInit()
├── cmd_install.go              # cmdInstall(), installVersion(), installInteractive(), installSingleRelease()
├── cmd_others.go               # cmdUse(), cmdCurrent(), cmdList(), cmdListRemote(), cmdUpdate(),
│                                #   cmdChannel(), cmdUninstall(), uninstallInteractive(), uninstallVersion(),
│                                #   cmdFetch(), fetchSHASUM(), switchTo()
├── go.mod
├── Makefile
├── install.sh                  # Bash installer (Linux/macOS)
├── install.ps1                 # PowerShell installer (Windows)
├── internal/
│   ├── github/client.go        # GitHub release client with 6h cache
│   ├── installer/installer.go  # Download, SHA256 verify, extract tar.gz/tgz/zip
│   ├── manager/
│   │   ├── manager.go          # Core state: versions, manifests, active channel
│   │   └── aliases.go          # Binary name resolution (legacy → modern mapping)
│   ├── platform/platform.go    # OS/arch/backend detection, asset suffix generation
│   ├── selfupdate/selfupdate.go # Self-update: download + replace running binary
│   ├── shim/shim.go            # Shim creation (Unix shell scripts / Windows .cmd files)
│   └── updater/updater.go      # Latest release fetch, semver comparison, asset lookup
└── CLAUDE.md                   # ← You are here
```

### Data Layout (`~/.llava` or `$LLAVA_HOME`)

```
~/.llava/
├── active              # File containing the active version ID string
├── channels.json       # Stable/beta channel → version ID mapping
├── cache/              # Cached GitHub releases JSON (6-hour TTL)
│   └── releases.json
├── shims/              # Shim scripts for each llama.cpp binary
│   ├── llama-cli
│   ├── llama-server
│   └── …               # (one per known binary, .cmd on Windows)
└── versions/
    └── b3412-cuda/     # Per-version directory named: {build}-{backend}
        ├── manifest.json  # Build, backend, channel, aliases, installed_at
        ├── llama-cli      # The actual binary (with legacy alias resolution)
        ├── llama-server
        └── …
```

---

## Command Reference

| Command                      | File             | Description                                                                                                                               |
| ---------------------------- | ---------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| `llava init`                   | `main.go`        | One-time setup: creates shims, auto-adds shims dir to shell PATH                                                                          |
| `llava install [version]`      | `cmd_install.go` | Install a build. Args: `latest`, `latest-beta`, `b3412`, or empty for interactive picker. Flags: `--backend`, `--use`, `--interactive/-i` |
| `llava use [version-id]`       | `cmd_others.go`  | Switch active version. Interactive picker if no arg. Flag: `--interactive/-i`                                                             |
| `llava current`                | `cmd_others.go`  | Print active version ID, channel, install date                                                                                            |
| `llava ls` / `llava list`        | `cmd_others.go`  | List installed versions with `▶` marker for active, channel tags                                                                          |
| `llava ls-remote`              | `cmd_others.go`  | List available GitHub releases. Flags: `--beta`, `--limit`                                                                                |
| `llava update`                 | `cmd_others.go`  | Check active version for newer release on same channel. Prompt to install. Flag: `--dry-run`                                              |
| `llava channel [stable\|beta]` | `cmd_others.go`  | Show current channel config or switch to stable/beta                                                                                      |
| `llava uninstall [version-id]` | `cmd_others.go`  | Remove a version. Interactive picker if no arg. Aliases: `remove`, `rm`                                                                   |
| `llava fetch`                  | `cmd_others.go`  | Force-refresh the GitHub releases cache                                                                                                   |
| `llava version`                | `main.go`        | Print llava version, prompt for self-update if newer available. Flag: `--skip-update-check`                                                 |

**Global flags** (on root command):

- `--skip-update-check` — skip the self-update check on `llava version`

---

## Key Packages & Responsibilities

### `internal/manager` — State management

**File**: `manager.go`

- **`Manager`** struct — wraps `~/.llava` path; methods for reading/writing state
- **`Version`** struct — `{ ID, Build, Backend, Channel, InstalledAt }`
- **`Manifest`** struct — `{ Build, Backend, Channel, Aliases, InstalledAt }` (stored as JSON per version)
- **`Channels`** struct — `{ Stable, Beta }` string pointers (maps channel names → version IDs)
- **Constants**: `ChannelStable = "stable"`, `ChannelBeta = "beta"`
- Key methods: `IsInstalled(id)`, `ListInstalled()`, `Active()`, `SwitchActiveAndChannel(id, ch)`, `Remove(id)`, `WriteManifest(id, manifest)`, `ReadManifest(id)`, `LoadChannels()`, `ClearStaleChannelReferences()`, `VersionsDir()`, `VersionDir(id)`, `ShimsDir()`, `CacheDir()`, `Home()`

**File**: `aliases.go`

- **`ResolveAliases(versionDir, binaryExt)`** — Maps canonical binary names → actual filenames. Handles llama.cpp's binary rename (b2900+): `main`→`llama-cli`, `server`→`llama-server`, `quantize`→`llama-quantize`, `embedding`→`llama-embedding`, `perplexity`→`llama-perplexity`, `imatrix`→`llama-imatrix`, `simple`→`llama-simple`, `tokenize`→`llama-tokenize`, `llama-bench`, `llama-run`
- Canonical set: `llama-cli`, `llama-server`, `llama-bench`, `llama-quantize`, `llama-embedding`, `llama-perplexity`, `llama-tokenize`, `llama-run`, `llama-simple`, `llama-imatrix`

### `internal/shim` — Binary shimming

**File**: `shim.go`

- **`shim.Manager`** — `{ shimsDir, llavaHome }`
- **`KnownBinaries`** — canonical list of 10 llama.cpp command names
- **`EnsureAll()`** — Creates shims for all known binaries; validates active version still exists
- **`Create(binaryName)`** — Generates a shim script:
  - **Unix**: `#!/bin/sh` script that reads `$LLAVA_HOME/active`, resolves binary path, exec's it
  - **Windows**: `.cmd` batch file with similar logic (`set /p VERSION < active`)
- **`List()`** — Returns installed shim names

### `internal/github` — GitHub releases client

**File**: `client.go`

- **`Client`** — wraps a cache directory
- **`Release`** struct — `{ TagName, Assets, PreRelease, PublishedAt }`
- **`Asset`** struct — `{ Name, URL, BrowserDownloadURL, Size }`
- Methods: `ListReleases()`, `LatestStable()`, `LatestBeta()`, `FindRelease(tag)`, `InvalidateCacheIfNeeded()`, `RefreshCache()`
- **Caching**: 6-hour TTL on `releases.json`; local JSON serialization

### `internal/installer` — Download & extract

**File**: `installer.go`

- **`Install(asset, destDir, progress)`** — Downloads, validates SHA256, extracts `.tar.gz`/`.tgz`/`.zip`
- **`Asset`** struct — `{ Name, URL, Size, SHA256 }`
- Progress callback: `func(downloaded, total int64)`
- On failure: cleans up partial `destDir`

### `internal/platform` — Platform detection

**File**: `platform.go`

- **`Info`** struct — `{ OS, Arch, Backend }`
- **`OS`**: `Linux`, `MacOS`, `Windows`
- **`Arch`**: `AMD64`, `ARM64`
- **`Backend`**: `cpu`, `cuda`, `metal`, `vulkan`, `rocm`, `sycl-fp16`, `sycl-fp32`, `openvino`
- **`Detect()`** — Auto-detects OS/arch/backend:
  - macOS → Metal (always)
  - Linux → CUDA if `nvidia-smi` exists, else Vulkan if `vulkaninfo`, else CPU
  - Windows → CUDA if `nvidia-smi`, else Vulkan
- **`DetectWithBackend(backendStr)`** — Override backend via string
- **`AssetSuffix()`** — Generates asset name fragment: `ubuntu-amd64`, `macos-arm64`, `win-cuda-cu12.2.0-x64`, etc.
- **`BinaryExt()`** — `""` on Unix, `".exe"` on Windows

### `internal/updater` — llava self-update metadata

**File**: `updater.go`

- **`LatestRelease()`** / **`LatestReleaseWithAssets()`** — Fetches from `api.github.com/repos/asertym/lvm/releases/latest`
- **`AssetForPlatform(release)`** — Finds asset matching `llava_<version>_<os>_<arch>` naming (`.exe` on Windows)
- **`SemverLess(a, b)`** — Compares semver strings (handles `v` prefix, missing patch)
- **`NewerAvailable(current)`** — Returns `(bool, latestTag, error)`

### `internal/selfupdate` — Self-update execution

**File**: `selfupdate.go`

- **`Update(downloadedURL, binPath)`** — Downloads latest binary and replaces running executable
  - **Unix**: Download to `.llava-update-tmp` → `chmod 0755` → `os.Rename` (atomic)
  - **Windows**: Download `.llava-new.exe` → write `.llava-update-helper.bat` → start batch (moves file, restarts llava, deletes temp)

---

## Version ID Construction

Version IDs follow the pattern: **`{build}-{backend}`**

Examples:

- `b3412-cuda`
- `b3500-metal`
- `b3600-arm64`

Built via `manager.VersionID(release.TagName, backendString)` where `release.TagName` is the GitHub tag like `b3412`.

---

## Environment Variables

| Variable   | Default  | Description                                                     |
| ---------- | -------- | --------------------------------------------------------------- |
| `LLAVA_HOME` | `~/.llava` | Base directory for llava state (versions, shims, cache, manifest) |

---

## Integrations

### GitHub (asertym/lvm)

- **llava releases**: Self-updates from `https://github.com/asertym/lvm/releases/latest`
- **llama.cpp releases**: All installs pull from `https://github.com/ggerganov/llama.cpp/releases` via `internal/github` client

### Shims

- Unix: Shell scripts in `~/.llava/shims/` — read `$LLAVA_HOME/active` to find binary
- Windows: `.cmd` batch files in `~/.llava/shims/` — read `%LLAVA_HOME%\active`
- `llava init` auto-appends shims dir to shell profile (`.bashrc`, `.zshrc`, `.bash_profile`, `.profile` on Unix; PowerShell `$env:PATH` via `HKCU:\Environment` on Windows)

---

## Development Workflow

### Build

```bash
make build            # Current platform
make build-windows    # Windows amd64
make build-linux      # Linux amd64
make build-macos      # macOS universal (amd64 + arm64)
```

### Run

```bash
go run . init
go run . install latest
```

### Test

No test files were found in the repository. Tests would be Go-style unit tests in `_test.go` files alongside their packages.

### Adding a new command

1. Add a `cmdXxx() *cobra.Command` function in the appropriate `cmd_*.go` file (or create a new one)
2. Register it in `root.AddCommand(...)` in `main.go`
3. Follow the existing pattern: cobra `Run` or `RunE`, huh for interactive prompts, color output

### Adding a new binary to the shim set

1. Add to `KnownBinaries` slice in `internal/shim/shim.go`
2. Add to `legacyNames` map in `internal/manager/aliases.go` with its fallback names

### Platform/backend support

1. Add constant to `Backend` type in `internal/platform/platform.go`
2. Add to `ParseBackend()` switch
3. Add to `detectBackend()` auto-detection logic (check for relevant CLI tool)
4. Add asset suffix mapping in `AssetSuffix()` switch

---

## Design Notes

- **Stateless shims**: Shims are generated at install/init time and read the `active` file at runtime. Switching versions is a single file write — no shim regeneration needed.
- **Atomic switches**: `llava use` writes `active` and `channels.json` together via `mgr.SwitchActiveAndChannel()`.
- **Binary compatibility**: The `ResolveAliases` system handles llama.cpp's 2024 binary rename (b2900+) gracefully by probing the filesystem.
- **Install safety**: Assets are validated (HTTP HEAD check before download) and SHA256 verified after download. Partial installs are cleaned up on failure.
- **Cache strategy**: GitHub releases are cached locally as JSON with a 6-hour TTL to reduce API rate-limit risk.
