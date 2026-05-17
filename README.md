# lvm — llama.cpp version manager

Manage multiple llama.cpp versions with one tool. Install, switch, and update
across stable and beta channels, with full GPU backend support.

## Quick start

```sh
# 1. Build
make build       # current platform
make all         # all platforms → dist/

# 2. Install
sudo cp lvm /usr/local/bin/

# 3. Set up
lvm init        # creates ~/.lvm, generates shims, prints PATH instruction

# 4. Add shims to PATH (one time) — printed by lvm init
export PATH="$HOME/.lvm/shims:$PATH"   # add to ~/.bashrc or ~/.zshrc

# 5. Install a version
lvm install latest
```

## Usage

```sh
# Install
lvm install latest              # latest stable
lvm install latest-beta         # latest pre-release
lvm install b3412               # specific build
lvm install latest --backend cuda
lvm install b3412 --backend vulkan

# Switch
lvm use b3412-cuda              # switch active version
lvm channel beta                # switch to beta channel (instant, no download)
lvm channel stable              # switch back to stable

# Info
lvm current                     # show active version
lvm ls                          # list installed versions
lvm ls-remote                   # list available on GitHub
lvm ls-remote --beta            # include pre-releases

# Update
lvm update                      # update to latest on current channel
lvm update --dry-run            # check without installing

# Remove
lvm uninstall b3200-cpu         # remove a version (must not be active)
```

## Backends

| Flag               | GPU                  | Linux | macOS | Windows |
| ------------------ | -------------------- | ----- | ----- | ------- |
| (default)          | auto-detected        | ✓     | ✓     | ✓       |
| `--backend cpu`    | None (CPU only)      | ✓     | ✓     | ✓       |
| `--backend cuda`   | NVIDIA               | ✓     |       | ✓       |
| `--backend metal`  | Apple GPU            |       | ✓     |         |
| `--backend vulkan` | AMD / NVIDIA / Intel | ✓     |       | ✓       |
| `--backend rocm`   | AMD                  | ✓     |       |         |

macOS auto-detects Metal. Linux auto-detects CUDA (nvidia-smi) then Vulkan.

## How it works

```
~/.lvm/
├── active                    ← plain text: "b3412-cuda"
├── channels.json             ← { "stable": "b3412-cuda", "beta": "b3500-cuda" }
├── shims/
│   ├── llama-cli             ← thin dispatcher (reads active, execs real binary)
│   ├── llama-server
│   └── ...
├── versions/
│   ├── b3412-cuda/
│   │   ├── llama-cli
│   │   ├── llama-server
│   │   └── manifest.json     ← binary aliases, build info
│   └── b3200-cpu/
│       ├── main              ← legacy binary name (pre-rename)
│       └── manifest.json     ← aliases: llama-cli → main
└── cache/
    └── releases_cache.json   ← GitHub API cache (1h TTL)
```

**Switching is instant.** `lvm use` writes one file (`active`). Shims read it
at runtime — no shell restart, no PATH changes, works in scripts and CI.

**Binary renames are handled automatically.** Older llama.cpp builds used `main`,
`server`, `quantize` etc. The manifest maps canonical names to real filenames,
so you always call `llama-cli` regardless of which build is active.

## Environment variables

| Variable   | Default  | Description                     |
| ---------- | -------- | ------------------------------- |
| `LVM_HOME` | `~/.lvm` | Override the lvm home directory |

## Windows

On Windows, shims are generated as `.cmd` batch files. Add the shims directory
to your PATH via System Properties → Environment Variables.

```
%USERPROFILE%\.lvm\shims
```

## Building from source

Requires Go 1.22+.

```sh
make deps    # go mod tidy
make build   # current platform
make all     # cross-compile for Linux + macOS + Windows → dist/
```
