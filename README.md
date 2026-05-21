# lvm — llama.cpp Version Manager

A cross-platform CLI tool for managing multiple [llama.cpp](https://github.com/ggerganov/llama.cpp) versions on your machine.

```bash
# Install latest stable version
lvm install latest

# Switch to a specific version
lvm use b3412-cuda

# List all installed versions
lvm ls
```

---

## What is lvm?

`lvm` is a lightweight version manager that simplifies working with multiple builds of llama.cpp. It handles:

- **Installation** of llama.cpp releases from GitHub
- **Version switching** between different builds (CPU, CUDA, Metal, Vulkan, etc.)
- **Channel management** between stable and beta releases
- **Automatic shims** for easy command invocation
- **Cross-platform support** (Windows, Linux, macOS)

Think of it like `nvm` (Node Version Manager) but for llama.cpp.

---

## Features

| Feature | Description |
|---------|-------------|
| **Multiple versions** | Install and switch between any llama.cpp release |
| **GPU backends** | Support for CPU, CUDA, Metal, Vulkan, ROCm, OpenVINO, SYCL |
| **Stable & Beta** | Separate channels for production and bleeding-edge builds |
| **One-command init** | Automatic PATH configuration, zero manual setup |
| **Auto-shims** | All llama.cpp binaries become accessible via simple commands |
| **Cross-platform** | Works on Windows, Linux, and macOS |
| **Cache** | Releases are cached for faster subsequent operations |
| **Clean uninstall** | Remove versions without leaving artifacts |

---

## Installation

### Official Installer (Recommended)

```bash
# Linux/macOS
curl -sSL https://github.com/YOURNAME/lvm/releases/latest/download/install.sh | sh

# Windows (PowerShell)
# Download from https://github.com/YOURNAME/lvm/releases and run install.ps1
```

### Manual Installation

```bash
# Download the binary for your platform
wget https://github.com/YOURNAME/lvm/releases/latest/download/lvm-linux-amd64

# Move to a location in your PATH
sudo mv lvm-linux-amd64 /usr/local/bin/lvm

# Initialize
lvm init
```

### Building from Source

```bash
git clone https://github.com/YOURNAME/lvm.git
cd lvm
go build -o lvm .
sudo mv lvm /usr/local/bin/lvm
lvm init
```

---

## Quick Start

```bash
# 1. Initialize (run once)
lvm init

# 2. Install latest stable version (auto-detects your platform)
lvm install latest

# 3. Start using llama.cpp commands
llama-cli --help
llama-quantize model.gguf q4_0.gguf
```

---

## Usage

### Install a Version

```bash
# Latest stable release
lvm install latest

# Latest beta/pre-release
lvm install latest-beta

# Specific build number
lvm install b3412

# With explicit GPU backend
lvm install latest --backend cuda
lvm install b3412 --backend vulkan
lvm install latest --backend metal
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
lvm use b3412-cuda

# Switch to the stable channel (uses last stable version)
lvm channel stable

# Switch to the beta channel (uses last beta version)
lvm channel beta
```

### List Versions

```bash
# List all locally installed versions
lvm ls

# List available releases on GitHub
lvm ls-remote

# Show current active version
lvm current
```

### Update

```bash
# Check for updates to the active version
lvm update

# Update to latest on the same channel
lvm update --backend cuda --use
```

### Uninstall

```bash
# Remove a specific version
lvm uninstall b3412-cuda
```

### Version Information

```bash
# Show lvm version
lvm version

# Show currently active version details
lvm current
```

---

## Examples

### Example 1: Setting Up a New Machine

```bash
# Clone the repo and build
git clone https://github.com/YOURNAME/lvm.git
cd lvm
go build -o lvm .
sudo mv lvm /usr/local/bin/lvm

# Initialize and install
lvm init
lvm install latest

# Verify
lvm current
llama-cli --version
```

### Example 2: Trying Different GPU Backends

```bash
# Try CUDA (if available)
lvm install latest --backend cuda
lvm use latest-cuda

# Fall back to Vulkan if CUDA fails
lvm uninstall latest-cuda
lvm install latest --backend vulkan
lvm use latest-vulkan

# CPU fallback
lvm uninstall latest-vulkan
lvm install latest --backend cpu
lvm use latest-cpu
```

### Example 3: Using Stable and Beta Channels

```bash
# Install and use stable (default)
lvm install latest
lvm use latest-cpu

# Later, try beta features
lvm install latest-beta
lvm channel beta

# Back to stable when ready
lvm channel stable
```

### Example 4: Managing Multiple Projects

```bash
# Project A uses older stable version
lvm use b3200-cuda

# Project B needs latest features
lvm use b3412-cuda

# Project C needs specific build
lvm install b3150
lvm use b3150-cpu
```

---

## Directory Structure

```
~/.lvm/
├── active              # Currently active version ID (e.g., "b3412-cuda")
├── channels.json       # Channel state (stable/beta → version IDs)
├── cache/              # Cached GitHub release data
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

`lvm` creates shell script wrappers (shims) for each llama.cpp binary:

```bash
# On Unix-like systems
llama-cli → ~/.lvm/shims/llama-cli
          → checks ~/.lvm/active
          → executes ~/.lvm/versions/<active>/llama-cli

# On Windows
llama-cli.cmd → %LVM_HOME%\shims\llama-cli.cmd
              → checks %LVM_HOME%\active
              → executes %LVM_HOME%\versions\<active>\llama-cli.exe
```

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

| Variable | Description |
|----------|-------------|
| `LVM_HOME` | Override default `~/.lvm` location |

### Custom Install Location

```bash
export LVM_HOME=/opt/lvm
lvm init
```

### Windows PATH

On Windows, `lvm init` automatically adds the shims directory to your user PATH via the Registry, ensuring it survives terminal restarts.

---

## Troubleshooting

### "no active version set"

```bash
# Solution: Install a version first
lvm install latest
lvm use latest-cpu
```

### "binary not found"

```bash
# Check active version
lvm current

# Re-initialize shims
rm ~/.lvm/shims/*
lvm init

# Or reinstall the version
lvm uninstall <version-id>
lvm install <version-id>
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
lvm install latest --backend cpu   # fall back to CPU

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

- Based on llama.cpp by [ggerganov](https://github.com/ggerganov)
- Inspired by tools like `nvm`, `rbenv`, `asdf`
