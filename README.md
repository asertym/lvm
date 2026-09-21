# llava — llama.cpp Version Manager

A cross-platform CLI tool for managing multiple [llama.cpp](https://github.com/ggml-org/llama.cpp) versions on your machine.

```bash
# Install latest stable version
llava install latest

# Switch to a specific version
llava use b3412-cuda

# Interactive version picker
llava use          # arrow-key selection
llava install      # arrow-key selection

# List all installed versions
llava ls
```

---

## What is llava?

`llava` is a lightweight version manager that simplifies working with multiple builds of llama.cpp. It handles:

- **Installation** of llama.cpp releases from GitHub
- **Version switching** between different builds (CPU, CUDA, Metal, Vulkan, etc.)
- **Channel management** between stable and beta releases
- **Automatic shims** for easy command invocation
- **Cross-platform support** (Windows, Linux, macOS)

Think of it like `nvm` (Node Version Manager) but for llama.cpp.

---

## Features

| Feature                | Description                                                        |
| ---------------------- | ------------------------------------------------------------------ |
| **Multiple versions**  | Install and switch between any llama.cpp release                   |
| **GPU backends**       | Support for CPU, CUDA, Metal, Vulkan, ROCm, OpenVINO, SYCL         |
| **Stable & Beta**      | Separate channels for production and bleeding-edge builds          |
| **Interactive picker** | Arrow-key selection UI for `use`, `install`, and `uninstall`       |
| **One-command init**   | Automatic PATH configuration, zero manual setup                    |
| **Auto-shims**         | All llama.cpp binaries become accessible via simple commands       |
| **Cross-platform**     | Works on Windows, Linux, and macOS                                 |
| **Cache**              | GitHub releases are cached for 6 hours to avoid repeated API calls |
| **Refresh**            | Run `llava fetch` to manually refresh cached data before TTL expires |
| **Clean uninstall**    | Remove versions without leaving artifacts                          |

---

## Installation

### Official Installer (Recommended)

```bash
# Linux/macOS
curl -sSL https://github.com/asertym/lvm/releases/latest/download/install.sh | sh

# Windows (PowerShell)
# Download from https://github.com/YOURNAME/llava/releases and run install.ps1
```

### Manual Installation

```bash
# Download the binary for your platform
wget https://github.com/asertym/lvm/releases/latest/download/llava-linux-amd64

# Move to a location in your PATH
sudo mv llava-linux-amd64 /usr/local/bin/llava

# Initialize
llava init
```

### Building from Source

```bash
git clone https://github.com/asertym/lvm.git
cd lvm
go build -o llava .
sudo mv llava /usr/local/bin/llava
llava init
```

---

## Quick Start

```bash
# 1. Initialize (run once)
llava init

# 2. Install a version (interactive picker by default)
llava install

# 3. Start using llama.cpp commands
llama-cli --help
llama-quantize model.gguf q4_0.gguf
```

---

## Usage

### Install a Version

```bash
# Latest stable release
llava install latest

# Latest beta/pre-release
llava install latest-beta

# Specific build number
llava install b3412

# With explicit GPU backend
llava install latest --backend cuda
llava install b3412 --backend vulkan
llava install latest --backend metal

# Interactive picker (default when no version given)
llava install        # picks from releases list
llava install -i     # same, explicit flag
llava install latest # non-interactive
```

**Available backends:**

- `cpu` — CPU-only build
- `cuda` — NVIDIA CUDA GPU acceleration
- `metal` — Apple Metal (macOS)
- `vulkan` — Vulkan API
- `rocm` — AMD ROCm
- `openvino` — Intel OpenVINO
- `sycl-fp16` / `sycl-fp16` — AMD SYCL

### Switch Versions

```bash
# Switch to a specific installed version
llava use b3412-cuda

# Interactive picker (default when no version given)
llava use            # picks from installed list
llava use -i         # same, explicit flag
llava use b3412-cuda # non-interactive

# Switch to the stable channel (uses last stable version)
llava channel stable

# Switch to the beta channel (uses last beta version)
llava channel beta
```

### List Versions

```bash
# List all locally installed versions
llava ls

# List available releases on GitHub
llava ls-remote

# Show current active version
llava current
```

### Fetch / Refresh Cache

```bash
# Manually refresh cached GitHub release data
llava fetch
```

By default, releases are cached for 6 hours. Use `llava fetch` to force a refresh before the cache expires.

### Update

```bash
# Check for updates to the active version
llava update
```

### Uninstall

```bash
# Remove a specific version
llava uninstall b3412-cuda

# Interactive picker (default when no version given)
llava uninstall      # picks from installed list
llava uninstall -i   # same, explicit flag
llava uninstall b3412-cuda # non-interactive
```

### Version Information

```bash
# Show llava version
llava version

# Show currently active version details
llava current
```

---

## Examples

### Example 1: Setting Up a New Machine

```bash
# Clone the repo and build
git clone https://github.com/asertym/lvm.git
cd lvm
go build -o llava .
sudo mv llava /usr/local/bin/llava

# Initialize and install
llava init
llava install latest

# Verify
llava current
llama-cli --version
```

### Example 2: Trying Different GPU Backends

```bash
# Try CUDA (if available)
llava install latest --backend cuda
llava use latest-cuda

# Fall back to Vulkan if CUDA fails
llava uninstall latest-cuda
llava install latest --backend vulkan
llava use latest-vulkan

# CPU fallback
llava uninstall latest-vulkan
llava install latest --backend cpu
llava use latest-cpu
```

### Example 3: Using Stable and Beta Channels

```bash
# Install and use stable (default)
llava install latest
llava use latest-cpu

# Later, try beta features
llava install latest-beta
llava channel beta

# Back to stable when ready
llava channel stable
```

### Example 4: Managing Multiple Projects

```bash
# Project A uses older stable version
llava use b3200-cuda

# Project B needs latest features
llava use b3412-cuda

# Project C needs specific build
llava install b3150
llava use b3150-cpu
```

### Example 5: Interactive Mode

```bash
# Pick an installed version with arrow keys
llava use

# Browse all releases and install one
llava install

# Remove a version (active version is protected from removal)
llava uninstall
```

---

## Directory Structure

```
~/.llava/
├── active              # Currently active version ID (e.g., "b3412-cuda")
├── channels.json       # Channel state (stable/beta → version IDs)
├── cache/              # Cached GitHub release data (6-hour TTL)
│   └── releases_cache.json
├── shims/              # Auto-generated shell scripts
│   ├── llama-cli
│   ├── llama-server
│   ├── llama-bench
│   ├── llama-quantize
│   └── ...
└── versions/           # Installed llama.cpp versions
    ├── b3412-cuda/
    │   ├── llama-cli
    │   ├── llama-server
    │   ├── ...
    │   └── manifest.json
    └── b3200-cpu/
        ├── main
        ├── ...
        └── manifest.json
```

---

## How It Works

### Version IDs

Versions are identified by unique IDs combining the build tag and backend:

```
b3412-cuda   # Build 3412 with CUDA backend
b3200-cpu    # Build 3200 with CPU backend
b3150-metal  # Build 3150 with Metal backend
```

### Shims

`llava` creates shell script wrappers (shims) for each llama.cpp binary:

```bash
# On Unix-like systems
llama-cli → ~/.llava/shims/llama-cli
          → checks ~/.llava/active
          → executes ~/.llava/versions/<active>/llama-cli

# On Windows
llama-cli.cmd → %LLAVA_HOME%\shims\llama-cli.cmd
              → checks %LLAVA_HOME%\active
              → executes %LLAVA_HOME%\versions\<active>\llama-cli.exe
```

### Interactive Picker

When called without a version argument (or with `-i`), `install`, `use`, and `uninstall` use a TUI picker built with [charmbracelet/huh](https://github.com/charmbracelet/huh):

- **Arrow keys** to navigate
- **Enter** to confirm
- **Ctrl+C** to abort cleanly
- Installed versions and beta releases are visually marked
- Active version cannot be removed in `uninstall`

The picker also adapts to non-TTY contexts (piped input, CI) by switching to accessible keyboard-only mode.

### Channel State

Two channels track the "default" version for each track:

```json
{
	"stable": "b3412-cuda",
	"beta": "b3500-cpu"
}
```

---

## Configuration

### Environment Variables

| Variable   | Description                        |
| ---------- | ---------------------------------- |
| `LLAVA_HOME` | Override default `~/.llava` location |

### Custom Install Location

```bash
export LLAVA_HOME=/opt/llava
llava init
```

### Windows PATH

On Windows, `llava init` automatically adds the shims directory to your user PATH via the Registry, ensuring it survives terminal restarts.

---

## Troubleshooting

### "no active version set"

```bash
# Solution: Install a version first
llava install latest
llava use latest-cpu
```

### "binary not found"

```bash
# Check active version
llava current

# Re-initialize shims
rm ~/.llava/shims/*
llava init

# Or reinstall the version
llava uninstall <version-id>
llava install <version-id>
```

### PATH not working

```bash
# Linux/macOS
source ~/.bashrc    # or ~/.zshrc

# Windows (PowerShell)
Get-ItemProperty -Path 'HKCU:\Environment' -Name PATH | Format-List
```

### GPU backend not detected

```bash
# Force a specific backend
llava install latest --backend cpu   # fall back to CPU

# Check available backends for your platform
nvidia-smi       # CUDA
vulkaninfo       # Vulkan
```

---

## Roadmap

- [ ] Version rollback/snapshots
- [ ] Backup and restore configurations
- [ ] Plugin system for custom backends
- [ ] GUI companion application
- [ ] Homebrew and Scoop packages

---

## Contributing

1. Fork the repository
2. Create a feature branch
3. Submit a pull request

See [CONTRIBUTING.md](CONTRIBUTING.md) for details.

---

## License

MIT License — see [LICENSE](LICENSE) for details.

---

## Credits

- Based on llama.cpp by [ggml-org](https://github.com/ggml-org)
- Inspired by tools like `nvm`, `n`, `rbenv`, `asdf`
