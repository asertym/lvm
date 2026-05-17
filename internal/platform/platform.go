package platform

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// OS represents a supported operating system.
type OS string

const (
	Linux   OS = "linux"
	MacOS   OS = "macos"
	Windows OS = "windows"
)

// Arch represents a CPU architecture.
type Arch string

const (
	AMD64 Arch = "amd64"
	ARM64 Arch = "arm64"
)

// Backend represents a GPU/compute backend.
type Backend string

const (
	BackendCPU      Backend = "cpu"
	BackendCUDA     Backend = "cuda"
	BackendMetal    Backend = "metal"
	BackendVulkan   Backend = "vulkan"
	BackendROCm     Backend = "rocm"
	BackendSYCLF16  Backend = "sycl-fp16"
	BackendSYCLF32  Backend = "sycl-fp32"
	BackendOpenVINO Backend = "openvino"
)

// Info holds detected platform information.
type Info struct {
	OS      OS
	Arch    Arch
	Backend Backend
}

// Detect returns the current platform info with auto-detected backend.
func Detect() (*Info, error) {
	info := &Info{}

	switch runtime.GOOS {
	case "linux":
		info.OS = Linux
	case "darwin":
		info.OS = MacOS
	case "windows":
		info.OS = Windows
	default:
		return nil, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	switch runtime.GOARCH {
	case "amd64":
		info.Arch = AMD64
	case "arm64":
		info.Arch = ARM64
	default:
		return nil, fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}

	info.Backend = detectBackend(info.OS)
	return info, nil
}

// DetectWithBackend returns platform info with an explicit backend override.
func DetectWithBackend(backendStr string) (*Info, error) {
	info, err := Detect()
	if err != nil {
		return nil, err
	}

	b, err := ParseBackend(backendStr)
	if err != nil {
		return nil, err
	}
	info.Backend = b
	return info, nil
}

// ParseBackend converts a string to a Backend, returning an error if unknown.
func ParseBackend(s string) (Backend, error) {
	switch strings.ToLower(s) {
	case "cpu", "":
		return BackendCPU, nil
	case "cuda":
		return BackendCUDA, nil
	case "metal":
		return BackendMetal, nil
	case "vulkan":
		return BackendVulkan, nil
	case "rocm":
		return BackendROCm, nil
	case "sycl-fp16":
		return BackendSYCLF16, nil
	case "sycl-fp32":
		return BackendSYCLF32, nil
	case "openvino":
		return BackendOpenVINO, nil
	default:
		return "", fmt.Errorf("unknown backend %q — valid: cpu, cuda, metal, vulkan, rocm, sycl-fp16, sycl-fp32, openvino", s)
	}
}

// detectBackend auto-detects the best available backend for the platform.
func detectBackend(os OS) Backend {
	switch os {
	case MacOS:
		// macOS builds always include Metal.
		return BackendMetal
	case Linux:
		// Try CUDA first (nvidia-smi), then Vulkan (vulkaninfo), then CPU.
		if commandExists("nvidia-smi") {
			return BackendCUDA
		}
		if commandExists("vulkaninfo") {
			return BackendVulkan
		}
		return BackendCPU
	case Windows:
		if commandExists("nvidia-smi") {
			return BackendCUDA
		}
		return BackendVulkan // Vulkan is widely available on Windows
	}
	return BackendCPU
}

// commandExists checks whether a command is available in PATH.
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// AssetSuffix returns the asset filename fragment used in llama.cpp GitHub releases.
// Examples: "ubuntu-x64", "macos-arm64", "win-cuda-cu12.2.0-x64"
func (i *Info) AssetSuffix() string {
	switch i.OS {
	case Linux:
		arch := assetArch(i.Arch)
		switch i.Backend {
		case BackendCUDA:
			return fmt.Sprintf("ubuntu-%s-cuda", arch)
		case BackendVulkan:
			return fmt.Sprintf("ubuntu-%s-vulkan", arch)
		case BackendROCm:
			return fmt.Sprintf("ubuntu-%s-rocm", arch)
		default:
			return fmt.Sprintf("ubuntu-%s", arch)
		}
	case MacOS:
		return fmt.Sprintf("macos-%s", assetArch(i.Arch))
	case Windows:
		arch := assetArch(i.Arch)
		switch i.Backend {
		case BackendCUDA:
			return fmt.Sprintf("win-cuda-%s", arch)
		case BackendVulkan:
			return fmt.Sprintf("win-vulkan-%s", arch)
		default:
			return fmt.Sprintf("win-%s", arch)
		}
	}
	return "unknown"
}

func assetArch(a Arch) string {
	switch a {
	case ARM64:
		return "arm64"
	default:
		return "x64"
	}
}

// BinaryExt returns the platform binary extension ("" on Unix, ".exe" on Windows).
func (i *Info) BinaryExt() string {
	if i.OS == Windows {
		return ".exe"
	}
	return ""
}

// String returns a human-readable summary of the platform.
func (i *Info) String() string {
	return fmt.Sprintf("%s/%s (%s)", i.OS, i.Arch, i.Backend)
}
