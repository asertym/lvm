package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"github.com/charmbracelet/huh"

	gh "lvm/internal/github"
	"lvm/internal/installer"
	"lvm/internal/manager"
	"lvm/internal/platform"
	"lvm/internal/shim"
)

func cmdInstall() *cobra.Command {
	var backendFlag string
	var useAfter bool
	var interactive bool

	cmd := &cobra.Command{
		Use:   "install [version]",
		Short: "Install a llama.cpp version",
		Long: `Install a llama.cpp version from GitHub releases.

Version can be:
  latest         latest stable release
  latest-beta    latest pre-release
  b3412          specific build number

Without a version argument, enters interactive mode (arrow-key selection).

Examples:
  lvm install latest
  lvm install b3412
  lvm install latest --backend vulkan
  lvm install   # interactive picker`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Interactive picker — no backend arg allowed.
			if interactive {
				if backendFlag != "" {
					return fmt.Errorf("--backend cannot be used with interactive mode")
				}
				return installInteractive(useAfter)
			}

			// Non-interactive: need a version argument.
			if len(args) == 0 {
				return fmt.Errorf("version argument required. Try: lvm install latest")
			}
			return installVersion(args[0], backendFlag, useAfter)
		},
	}

	cmd.Flags().StringVar(&backendFlag, "backend", "", "GPU backend: cpu, cuda, metal, vulkan, rocm, sycl-fp16, sycl-fp32, openvino")
	cmd.Flags().BoolVar(&useAfter, "use", false, "Switch to this version immediately after install")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactive arrow-key selection")
	return cmd
}

// installVersion is the non-interactive install flow (kept for reuse and backward compat).
func installVersion(versionArg, backendFlag string, useAfter bool) error {
	// Resolve platform.
	var plat *platform.Info
	var err error
	if backendFlag != "" {
		plat, err = platform.DetectWithBackend(backendFlag)
	} else {
		plat, err = platform.Detect()
	}
	if err != nil {
		return err
	}

	fmt.Printf("Platform: %s\n", plat)

	// Resolve the requested version to a GitHub release.
	client := gh.NewClient(mgr.CacheDir())

	var release *gh.Release
	switch strings.ToLower(versionArg) {
	case "latest":
		fmt.Print("Fetching latest stable release... ")
		release, err = client.LatestStable()
	case "latest-beta", "beta":
		fmt.Print("Fetching latest beta release... ")
		release, err = client.LatestBeta()
	default:
		fmt.Printf("Looking up release %s... ", versionArg)
		release, err = client.FindRelease(versionArg)
	}
	if err != nil {
		fmt.Println()
		return fmt.Errorf("could not find release: %w", err)
	}
	fmt.Printf("found %s\n", release.TagName)

	// Determine channel.
	ch := manager.ChannelStable
	if release.PreRelease {
		ch = manager.ChannelBeta
	}

	// Build the version ID.
	versionID := manager.VersionID(release.TagName, string(plat.Backend))

	// Already installed?
	if mgr.IsInstalled(versionID) {
		yellow := color.New(color.FgYellow).SprintFunc()
		fmt.Printf("%s %s is already installed\n", yellow("→"), versionID)
		if useAfter {
			return switchTo(versionID, ch)
		}
		return nil
	}

	// Find matching asset.
	suffix := plat.AssetSuffix()
	fmt.Printf("Looking for asset matching %q...\n", suffix)
	asset, err := release.FindAsset(suffix)
	if err != nil {
		return err
	}
	fmt.Printf("Asset: %s (%.1f MB)\n", asset.Name, float64(asset.Size)/1e6)

	// Resolve checksum if available (best-effort).
	sha256 := ""
	if sumAsset := release.FindSHASUM(asset.Name); sumAsset != nil {
		sha256, _ = fetchSHASUM(sumAsset.BrowserDownloadURL, asset.Name)
	}

	// Download and extract.
	destDir := mgr.VersionDir(versionID)
	fmt.Printf("Installing to %s...\n", destDir)

	progress := makeProgressPrinter()
	err = installer.Install(&installer.Asset{
		Name:   asset.Name,
		URL:    asset.BrowserDownloadURL,
		Size:   asset.Size,
		SHA256: sha256,
	}, destDir, progress)
	if err != nil {
		// Clean up partial install.
		os.RemoveAll(destDir)
		return fmt.Errorf("installation failed: %w", err)
	}
	fmt.Println()

	// Resolve binary aliases and write manifest.
	aliases := manager.ResolveAliases(destDir, plat.BinaryExt())
	manifest := &manager.Manifest{
		Build:       release.TagName,
		Backend:     string(plat.Backend),
		Channel:     ch,
		Aliases:     aliases,
		InstalledAt: time.Now(),
	}
	if err := mgr.WriteManifest(versionID, manifest); err != nil {
		return fmt.Errorf("manifest write failed: %w", err)
	}

	// Ensure shims exist for any new binaries.
	shimMgr := shim.NewManager(mgr.ShimsDir(), mgr.Home())
	for canonical := range aliases {
		_ = shimMgr.Ensure(canonical)
	}

	green := color.New(color.FgGreen, color.Bold).SprintFunc()
	fmt.Printf("%s Installed %s\n", green("✓"), versionID)

	// Switch to it if requested or if nothing is active yet.
	if useAfter || mgr.Active() == "" {
		return switchTo(versionID, ch)
	}

	fmt.Printf("  Run %s to switch to it\n", color.New(color.Bold).Sprintf("lvm use %s", versionID))
	return nil
}

// installInteractive shows a huh-based picker for available releases.
func installInteractive(useAfter bool) error {
	client := gh.NewClient(mgr.CacheDir())
	releases, err := client.ListReleases()
	if err != nil {
		return fmt.Errorf("could not fetch releases: %w", err)
	}

	if len(releases) == 0 {
		return fmt.Errorf("no releases found")
	}

	// Build options with installed status.
	installedVersions := make(map[string]bool)
	allInstalled, _ := mgr.ListInstalled()
	for _, v := range allInstalled {
		installedVersions[v.Build] = true
	}

	type installChoice struct {
		tag   string
		label string
	}

	choices := make([]installChoice, 0, len(releases))
	for _, r := range releases {
		label := r.TagName
		if r.PreRelease {
			yellow := color.New(color.FgYellow).SprintFunc()
			label = fmt.Sprintf("%s  %s", r.TagName, yellow("beta"))
		}
		if installedVersions[r.TagName] {
			green := color.New(color.FgGreen).SprintFunc()
			label = fmt.Sprintf("%s  %s", label, green("installed"))
		}
		choices = append(choices, installChoice{tag: r.TagName, label: label})
	}

	// Limit to most recent releases to avoid overwhelming the list.
	if len(choices) > 50 {
		choices = choices[:50]
	}

	options := make([]huh.Option[installChoice], len(choices))
	for i, c := range choices {
		options[i] = huh.NewOption(c.label, c)
	}

	var selectedChoice installChoice
	a := isatty.IsTerminal(os.Stdin.Fd())

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[installChoice]().
				Title("Select a release to install").
				Description("Arrow keys to navigate, Enter to confirm. Installed builds are marked").
				Options(options...).Value(&selectedChoice),
		),
	)
	if a {
		form = form.WithAccessible(false)
	}

	if err := form.Run(); err != nil {
		if err == huh.ErrUserAborted {
			return nil
		}
		return fmt.Errorf("selection aborted: %w", err)
	}

	// Resolve the selected release.
	client2 := gh.NewClient(mgr.CacheDir())
	release, err := client2.FindRelease(selectedChoice.tag)
	if err != nil {
		return fmt.Errorf("release %s not found: %w", selectedChoice.tag, err)
	}

	// Determine backend.
	backendToUse := ""
	if useAfter {
		// If --use was passed interactively, ask for backend first.
		backends := []string{"cpu", "cuda", "metal", "vulkan", "rocm"}
		beOptions := make([]huh.Option[string], 0, len(backends))
		for _, b := range backends {
			beOptions = append(beOptions, huh.NewOption(b, b))
		}
		var selectedBackend string
		backendForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select backend").
					Description("GPU/CPU backend for this build").
					Options(beOptions...).Value(&selectedBackend),
			),
		)
		if a {
			backendForm = backendForm.WithAccessible(false)
		}
		if err := backendForm.Run(); err != nil {
			if err == huh.ErrUserAborted {
				return nil
			}
			return fmt.Errorf("backend selection aborted: %w", err)
		}
		backendToUse = selectedBackend
	}
	_ = backendToUse // we don't auto-use in the current flow, just install

	// Install with detected backend.
	return installSingleRelease(release, useAfter)
}

// installSingleRelease installs a single release using auto-detected backend.
func installSingleRelease(release *gh.Release, useAfter bool) error {
	// Resolve platform with auto-detection.
	plat, err := platform.Detect()
	if err != nil {
		return err
	}

	// Determine channel.
	ch := manager.ChannelStable
	if release.PreRelease {
		ch = manager.ChannelBeta
	}

	// Build the version ID.
	versionID := manager.VersionID(release.TagName, string(plat.Backend))

	// Already installed?
	if mgr.IsInstalled(versionID) {
		yellow := color.New(color.FgYellow).SprintFunc()
		fmt.Printf("%s %s is already installed\n", yellow("→"), versionID)
		if useAfter {
			return switchTo(versionID, ch)
		}
		return nil
	}

	// Find matching asset.
	suffix := plat.AssetSuffix()
	fmt.Printf("Looking for asset matching %q...\n", suffix)
	asset, err := release.FindAsset(suffix)
	if err != nil {
		return err
	}
	fmt.Printf("Asset: %s (%.1f MB)\n", asset.Name, float64(asset.Size)/1e6)

	// Resolve checksum if available (best-effort).
	sha256 := ""
	if sumAsset := release.FindSHASUM(asset.Name); sumAsset != nil {
		sha256, _ = fetchSHASUM(sumAsset.BrowserDownloadURL, asset.Name)
	}

	// Download and extract.
	destDir := mgr.VersionDir(versionID)
	fmt.Printf("Installing to %s...\n", destDir)

	progress := makeProgressPrinter()
	err = installer.Install(&installer.Asset{
		Name:   asset.Name,
		URL:    asset.BrowserDownloadURL,
		Size:   asset.Size,
		SHA256: sha256,
	}, destDir, progress)
	if err != nil {
		// Clean up partial install.
		os.RemoveAll(destDir)
		return fmt.Errorf("installation failed: %w", err)
	}
	fmt.Println()

	// Resolve binary aliases and write manifest.
	aliases := manager.ResolveAliases(destDir, plat.BinaryExt())
	manifest := &manager.Manifest{
		Build:       release.TagName,
		Backend:     string(plat.Backend),
		Channel:     ch,
		Aliases:     aliases,
		InstalledAt: time.Now(),
	}
	if err := mgr.WriteManifest(versionID, manifest); err != nil {
		return fmt.Errorf("manifest write failed: %w", err)
	}

	// Ensure shims exist for any new binaries.
	shimMgr := shim.NewManager(mgr.ShimsDir(), mgr.Home())
	for canonical := range aliases {
		_ = shimMgr.Ensure(canonical)
	}

	green := color.New(color.FgGreen, color.Bold).SprintFunc()
	fmt.Printf("%s Installed %s\n", green("✓"), versionID)

	// Switch to it if requested or if nothing is active yet.
	if useAfter || mgr.Active() == "" {
		return switchTo(versionID, ch)
	}

	fmt.Printf("  Run %s to switch to it\n", color.New(color.Bold).Sprintf("lvm use %s", versionID))
	return nil
}

// makeProgressPrinter returns a progress callback that prints a simple progress bar.
func makeProgressPrinter() func(int64, int64) {
	lastPct := -1
	return func(downloaded, total int64) {
		if total <= 0 {
			return
		}
		pct := int(downloaded * 100 / total)
		if pct == lastPct {
			return
		}
		lastPct = pct
		bar := strings.Repeat("█", pct/5) + strings.Repeat("░", 20-pct/5)
		fmt.Printf("\r  [%s] %d%%", bar, pct)
	}
}

// fetchSHASUM downloads a SHASUM file and extracts the checksum for a given asset.
func fetchSHASUM(url, assetName string) (string, error) {
	// Simple: download the file, look for the asset name in each line.
	tmpPath := os.TempDir() + "/lvm-shasums"
	if err := gh.DownloadFile(url, tmpPath, nil); err != nil {
		return "", err
	}
	defer os.Remove(tmpPath)

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 2 && strings.EqualFold(parts[1], assetName) {
			return parts[0], nil
		}
		// Some formats use "hash  *filename"
		if len(parts) >= 2 && strings.EqualFold(strings.TrimPrefix(parts[1], "*"), assetName) {
			return parts[0], nil
		}
	}
	return "", fmt.Errorf("checksum for %s not found in SHASUMS", assetName)
}
