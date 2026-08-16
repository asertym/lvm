package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	gh "lvm/internal/github"
	"lvm/internal/manager"
)

// --- lvm use ---

func cmdUse() *cobra.Command {
	var interactive bool

	cmd := &cobra.Command{
		Use:   "use [version-id]",
		Short: "Switch to an installed version",
		Long: `Switch the active llama.cpp version.

Without an argument, enters interactive mode (arrow-key selection).
With a version-id, switches directly.

Examples:
  lvm use b3412-cuda
  lvm use   # interactive picker
`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			versions, err := mgr.ListInstalled()
			if err != nil {
				return err
			}
			if len(versions) == 0 {
				return fmt.Errorf("no versions installed. Run: lvm install latest")
			}

			// Explicit version argument — fast path.
			if len(args) > 0 && !interactive {
				return useVersion(args[0])
			}

			// Interactive picker.
			return useInteractive(versions)
		},
	}

	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactive arrow-key selection")
	return cmd
}

// useInteractive shows a huh-based arrow-key picker for installed versions.
func useInteractive(versions []manager.Version) error {
	var selectedID string
	options := make([]huh.Option[string], len(versions))
	for i, v := range versions {
		label := v.ID
		if v.Channel == manager.ChannelBeta {
			yellow := color.New(color.FgYellow).SprintFunc()
			label = fmt.Sprintf("%s  %s", v.ID, yellow("beta"))
		}
		options[i] = huh.NewOption(label, v.ID)
	}

	a := isatty.IsTerminal(os.Stdin.Fd())
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select a version to use").
				Description("Arrow keys to navigate, Enter to confirm").
				Options(options...).Value(&selectedID),
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

	// Validate selection.
	manifest, err := mgr.ReadManifest(selectedID)
	ch := manager.ChannelStable
	if err == nil {
		ch = manifest.Channel
	}
	return switchTo(selectedID, ch)
}

// useVersion handles the non-interactive path (called from both explicit arg and programmatic).
func useVersion(id string) error {
	if !mgr.IsInstalled(id) {
		return fmt.Errorf(
			"%q is not installed\nRun 'lvm install %s' to install it, or 'lvm ls' to see installed versions",
			id, id,
		)
	}
	manifest, err := mgr.ReadManifest(id)
	ch := manager.ChannelStable
	if err == nil {
		ch = manifest.Channel
	}
	return switchTo(id, ch)
}

// switchTo atomically updates both active and channel state, then prints confirmation.
func switchTo(id string, ch manager.Channel) error {
	if err := mgr.SwitchActiveAndChannel(id, ch); err != nil {
		return fmt.Errorf("could not switch state: %w", err)
	}

	green := color.New(color.FgGreen, color.Bold).SprintFunc()
	fmt.Printf("%s Now using %s (%s channel)\n", green("✓"), id, ch)
	return nil
}

// --- lvm current ---

func cmdCurrent() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Show the active llama.cpp version",
		RunE: func(cmd *cobra.Command, args []string) error {
			active := mgr.Active()
			if active == "" {
				fmt.Println("No active version. Run: lvm install latest")
				return nil
			}

			manifest, err := mgr.ReadManifest(active)
			if err != nil {
				fmt.Println(active)
				return nil
			}

			bold := color.New(color.Bold).SprintFunc()
			dim := color.New(color.Faint).SprintFunc()
			fmt.Printf("%s  %s  %s\n",
				bold(active),
				dim("channel:"+string(manifest.Channel)),
				dim("installed:"+manifest.InstalledAt.Format("2006-01-02")),
			)
			return nil
		},
	}
}

// --- lvm ls ---

func cmdList() *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List installed versions",
		RunE: func(cmd *cobra.Command, args []string) error {
			versions, err := mgr.ListInstalled()
			if err != nil {
				return err
			}

			if len(versions) == 0 {
				fmt.Println("No versions installed. Run: lvm install latest")
				return nil
			}

			active := mgr.Active()
			channels, _ := mgr.LoadChannels()

			green := color.New(color.FgGreen, color.Bold).SprintFunc()
			dim := color.New(color.Faint).SprintFunc()
			yellow := color.New(color.FgYellow).SprintFunc()

			fmt.Println()
			for _, v := range versions {
				prefix := "  "
				suffix := ""

				if v.ID == active {
					prefix = green("▶ ")
				}

				tags := []string{}
				if channels != nil {
					if channels.Stable == v.ID {
						tags = append(tags, yellow("stable"))
					}
					if channels.Beta == v.ID {
						tags = append(tags, yellow("beta"))
					}
				}

				if len(tags) > 0 {
					suffix = "  " + dim("["+strings.Join(tags, ", ")+"]")
				}

				installed := ""
				if !v.InstalledAt.IsZero() {
					installed = dim("  " + v.InstalledAt.Format("2006-01-02"))
				}

				fmt.Printf("%s%s%s%s\n", prefix, v.ID, suffix, installed)
			}
			fmt.Println()
			return nil
		},
	}
}

// --- lvm ls-remote ---

func cmdListRemote() *cobra.Command {
	var showBeta bool
	var limit int

	cmd := &cobra.Command{
		Use:   "ls-remote",
		Short: "List available releases on GitHub",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Print("Fetching releases from GitHub... ")
			client := gh.NewClient(mgr.CacheDir())
			releases, err := client.ListReleases()
			if err != nil {
				fmt.Println()
				return err
			}
			fmt.Printf("found %d releases\n\n", len(releases))

			dim := color.New(color.Faint).SprintFunc()
			yellow := color.New(color.FgYellow).SprintFunc()

			// Build a set of installed build tags for O(1) lookup.
			installedBuilds := make(map[string]bool)
			installedVersions, _ := mgr.ListInstalled()
			for _, v := range installedVersions {
				installedBuilds[v.Build] = true
			}

			count := 0
			for _, r := range releases {
				if !showBeta && r.PreRelease {
					continue
				}
				if count >= limit {
					break
				}

				label := ""
				if r.PreRelease {
					label = yellow("  beta")
				}
				date := ""
				if r.PublishedAt != "" && len(r.PublishedAt) >= 10 {
					date = dim("  " + r.PublishedAt[:10])
				}

				installed := ""
				if installedBuilds[r.TagName] {
					installed = color.New(color.FgGreen).Sprint("  ✓ installed")
				}

				fmt.Printf("  %s%s%s%s\n", r.TagName, label, date, installed)
				count++
			}
			fmt.Println()
			return nil
		},
	}

	cmd.Flags().BoolVar(&showBeta, "beta", false, "Include pre-release builds")
	cmd.Flags().IntVar(&limit, "limit", 20, "Number of releases to show")
	return cmd
}

// --- lvm update ---

func cmdUpdate() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update active version to latest on its channel",
		RunE: func(cmd *cobra.Command, args []string) error {
			active := mgr.Active()
			if active == "" {
				return fmt.Errorf("no active version — run 'lvm install latest' first")
			}

			manifest, err := mgr.ReadManifest(active)
			if err != nil {
				return fmt.Errorf("cannot read active version manifest: %w", err)
			}

			client := gh.NewClient(mgr.CacheDir())
			_ = client.InvalidateCacheIfNeeded() // only invalidate if cache is stale or missing

			var release *gh.Release
			if manifest.Channel == manager.ChannelBeta {
				release, err = client.LatestBeta()
			} else {
				release, err = client.LatestStable()
			}
			if err != nil {
				return err
			}

			targetID := manager.VersionID(release.TagName, manifest.Backend)

			if targetID == active {
				green := color.New(color.FgGreen).SprintFunc()
				fmt.Printf("%s Already on latest %s (%s)\n", green("✓"), active, manifest.Channel)
				return nil
			}

			fmt.Printf("Update available: %s → %s\n", active, targetID)
			if dryRun {
				fmt.Println("(dry run — no changes made)")
				return nil
			}

			// Prompt user to install the update.
			var choice string
			a := isatty.IsTerminal(os.Stdin.Fd())
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title("Install update?").
						Description("To install a specific version, run: lvm install").
						Options(
							huh.NewOption("get update", "get update"),
							huh.NewOption("no thanks", "no thanks"),
						).Value(&choice),
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

			if choice == "no thanks" {
				return nil
			}

			// Install the update.
			return installVersion(release.TagName, manifest.Backend, true)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be updated without making changes")
	return cmd
}

// --- lvm channel ---

func cmdChannel() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channel [stable|beta]",
		Short: "Show or switch the active release channel",
		Long: `Show the current channel or switch between stable and beta.

Switching channels instantly activates the version that was last used
on that channel (no download needed if it was previously installed).

Examples:
  lvm channel              show current channel info
  lvm channel stable       switch to stable channel
  lvm channel beta         switch to beta channel`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			channels, err := mgr.LoadChannels()
			if err != nil {
				return err
			}

			if len(args) == 0 {
				// Show current state.
				active := mgr.Active()
				manifest, _ := mgr.ReadManifest(active)
				ch := "unknown"
				if manifest != nil {
					ch = string(manifest.Channel)
				}

				bold := color.New(color.Bold).SprintFunc()
				dim := color.New(color.Faint).SprintFunc()
				fmt.Printf("\nActive channel: %s\n", bold(ch))
				fmt.Printf("  stable → %s\n", valueOrNone(channels.Stable))
				fmt.Printf("  beta   → %s\n\n", valueOrNone(channels.Beta))
				_ = dim
				return nil
			}

			target := strings.ToLower(args[0])
			switch target {
			case "stable":
				if channels.Stable == "" {
					return fmt.Errorf("no stable version installed — run 'lvm install latest' first")
				}
				if !mgr.IsInstalled(channels.Stable) {
					return fmt.Errorf("stable version %q is no longer installed — run 'lvm install latest'", channels.Stable)
				}
				return switchTo(channels.Stable, manager.ChannelStable)

			case "beta":
				if channels.Beta == "" {
					return fmt.Errorf("no beta version installed — run 'lvm install latest-beta' first")
				}
				if !mgr.IsInstalled(channels.Beta) {
					return fmt.Errorf("beta version %q is no longer installed — run 'lvm install latest-beta'", channels.Beta)
				}
				return switchTo(channels.Beta, manager.ChannelBeta)

			default:
				return fmt.Errorf("unknown channel %q — use 'stable' or 'beta'", target)
			}
		},
	}
	return cmd
}

// --- lvm uninstall ---

func cmdUninstall() *cobra.Command {
	var interactive bool

	cmd := &cobra.Command{
		Use:     "uninstall [version-id]",
		Aliases: []string{"remove", "rm"},
		Short:   "Remove an installed version",
		Long: `Remove an installed llama.cpp version.

Without an argument, enters interactive mode (arrow-key selection).
With a version-id, removes it directly.

Examples:
  lvm uninstall b3412-cuda
  lvm uninstall   # interactive picker
`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			versions, err := mgr.ListInstalled()
			if err != nil {
				return err
			}

			// Explicit version argument — fast path.
			if len(args) > 0 && !interactive {
				return uninstallVersion(args[0])
			}

			return uninstallInteractive(versions)
		},
	}

	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactive arrow-key selection")
	return cmd
}

// uninstallInteractive shows a huh-based picker and confirms removal.
func uninstallInteractive(versions []manager.Version) error {
	var selectedID string
	active := mgr.Active()

	// Exclude the active version entirely from the picker.
	options := make([]huh.Option[string], 0, len(versions)-1)
	for _, v := range versions {
		if v.ID == active {
			continue
		}
		options = append(options, huh.NewOption(v.ID, v.ID))
	}

	if len(options) == 0 {
		return fmt.Errorf("no removable versions (active version %q cannot be removed)", active)
	}

	a := isatty.IsTerminal(os.Stdin.Fd())
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select a version to remove").
				Description("Arrow keys to navigate, Enter to confirm").
				Options(options...).Value(&selectedID),
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

	if err := mgr.Remove(selectedID); err != nil {
		return err
	}
	green := color.New(color.FgGreen, color.Bold).SprintFunc()
	fmt.Printf("%s Removed %s\n", green("✓"), selectedID)
	return nil
}

// uninstallVersion handles the non-interactive path.
func uninstallVersion(id string) error {
	if err := mgr.Remove(id); err != nil {
		return err
	}
	// Clear stale channel references and active pointer if they pointed here.
	if err := mgr.ClearStaleChannelReferences(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not clear stale references: %v\n", err)
	}
	green := color.New(color.FgGreen, color.Bold).SprintFunc()
	fmt.Printf("%s Removed %s\n", green("✓"), id)
	return nil
}

// --- lvm fetch ---

func cmdFetch() *cobra.Command {
	return &cobra.Command{
		Use:   "fetch",
		Short: "Fetch and cache the latest GitHub releases",
		Long: `Fetch the latest releases from GitHub and save them to the local cache.

The cache is valid for 6 hours by default. Use this command to force
a fresh fetch before the cache expires, or when you suspect stale data.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := gh.NewClient(mgr.CacheDir())
			fmt.Print("Fetching releases from GitHub... ")
			if err := client.RefreshCache(); err != nil {
				return fmt.Errorf("failed to fetch: %w", err)
			}
			green := color.New(color.FgGreen, color.Bold).SprintFunc()
			fmt.Printf("%s cache updated\n", green("✓"))
			return nil
		},
	}
}

func valueOrNone(s string) string {
	if s == "" {
		return color.New(color.Faint).Sprint("(none)")
	}
	return s
}

// --- lvm uninstall-self ---

func cmdUninstallSelf() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "uninstall-self",
		Short: "Uninstall lvm completely from your machine",
		Long: `Completely remove lvm from your system.

This will:
  - Delete the lvm home directory (~/.lvm or $LVM_HOME)
  - Remove the lvm binary from standard locations
  - Remove lvm PATH entries from shell profiles (Unix) or user PATH (Windows)

The installed llama.cpp versions will also be removed.

Use --yes to skip the confirmation prompt.
`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Determine lvm home.
			lvmHome, err := lvmHome()
			if err != nil {
				return fmt.Errorf("cannot determine lvm home: %w", err)
			}

			// 2. Ask for confirmation.
			if !force {
				var confirm string
				a := isatty.IsTerminal(os.Stdin.Fd())
				form := huh.NewForm(
					huh.NewGroup(
						huh.NewSelect[string]().
							Title("Uninstall lvm?").
							Description("This will remove all llama.cpp versions, shims, and configuration.").
							Options(
								huh.NewOption("uninstall", "yes"),
								huh.NewOption("cancel", "no"),
							).Value(&confirm),
					),
				)
				if a {
					form = form.WithAccessible(false)
				}

				if err := form.Run(); err != nil {
					if err == huh.ErrUserAborted {
						return nil
					}
					return fmt.Errorf("confirmation aborted: %w", err)
				}

				if confirm != "yes" {
					return nil
				}
			}

			green := color.New(color.FgGreen, color.Bold).SprintFunc()
			yellow := color.New(color.FgYellow).SprintFunc()

			// 3. Remove lvm home directory.
			if err := os.RemoveAll(lvmHome); err != nil {
				// Don't abort — try to clean up the rest.
				fmt.Fprintf(os.Stderr, "%s Could not remove %s: %v\n", yellow("⚠"), lvmHome, err)
			} else {
				fmt.Printf("%s Removed %s\n", green("✓"), lvmHome)
			}

			// 4. Remove lvm binary.
			removeLvmBinary()

			// 5. Clean PATH entries.
			cleanPathForUninstall()

			fmt.Println()
			bold := color.New(color.Bold).SprintFunc()
			fmt.Printf("%s lvm has been uninstalled.\n", bold("Done"))
			fmt.Println()

			// Warn about custom LVM_HOME.
			if os.Getenv("LVM_HOME") != "" && os.Getenv("LVM_HOME") != lvmHome {
				fmt.Printf("%s Warning: $LVM_HOME is set to a custom path that was not cleaned up:\n", yellow("⚠"))
				fmt.Printf("   %s\n", os.Getenv("LVM_HOME"))
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "yes", false, "Skip confirmation prompt")
	return cmd
}

// removeLvmBinary attempts to remove the lvm binary from standard install locations.
func removeLvmBinary() {
	locations := []string{
		"/usr/local/bin/lvm",
		"/usr/bin/lvm",
		"/opt/local/bin/lvm",
		"/usr/sbin/lvm",
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		locations = append(locations,
			filepath.Join(home, "bin", "lvm"),
		)
	}
	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			if err := os.Remove(loc); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not remove %s: %v\n", loc, err)
			} else {
				fmt.Printf("✓ Removed %s\n", loc)
			}
		}
	}
	// Windows: %USERPROFILE%\bin\lvm.exe
	if runtime.GOOS == "windows" && home != "" {
		winLoc := filepath.Join(home, "bin", "lvm.exe")
		if _, err := os.Stat(winLoc); err == nil {
			if err := os.Remove(winLoc); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not remove %s: %v\n", winLoc, err)
			} else {
				fmt.Printf("✓ Removed %s\n", winLoc)
			}
		}
	}
}

// cleanPathForUninstall removes lvm-related entries from shell profiles (Unix)
// or the Windows user PATH registry entry.
func cleanPathForUninstall() {
	if runtime.GOOS == "windows" {
		cleanWindowsPath()
		return
	}
	cleanUnixShellProfiles()
}

// cleanUnixShellProfiles removes lvm-related lines from found shell profile files.
func cleanUnixShellProfiles() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	lvmHome, _ := lvmHome()
	shimLine := "export PATH=\"" + filepath.Join(lvmHome, "shims") + ":$PATH\""
	profiles := []string{".zshrc", ".bashrc", ".bash_profile", ".profile"}
	for _, profile := range profiles {
		profilePath := filepath.Join(home, profile)
		data, err := os.ReadFile(profilePath)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		var cleaned []string
		removed := false
		for _, line := range lines {
			if strings.TrimSpace(line) == shimLine {
				removed = true
				continue
			}
			// install.sh adds: export PATH="/usr/local/bin:$PATH"
			if strings.TrimSpace(line) == "export PATH=\"/usr/local/bin:$PATH\"" {
				removed = true
				continue
			}
			cleaned = append(cleaned, line)
		}
		if removed {
			if err := os.WriteFile(profilePath, []byte(strings.Join(cleaned, "\n")), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not clean %s: %v\n", profile, err)
			} else {
				fmt.Printf("✓ Cleaned ~/%s\n", profile)
			}
		}
	}
}

// cleanWindowsPath removes lvm-related paths from the user PATH registry entry.
func cleanWindowsPath() {
	lvmHome, _ := lvmHome()
	toRemove := []string{
		filepath.Join(lvmHome, "shims"),
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		toRemove = append(toRemove, filepath.Join(home, "bin"))
	}
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		`[Environment]::GetEnvironmentVariable('PATH', 'User')`).CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not read user PATH: %v\n", err)
		return
	}
	currentPath := strings.TrimSpace(string(out))
	parts := strings.Split(currentPath, ";")
	var kept []string
	removed := false
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		skip := false
		for _, remove := range toRemove {
			if p == remove {
				skip = true
				removed = true
				break
			}
		}
		if !skip {
			kept = append(kept, p)
		}
	}
	if !removed {
		return
	}
	newPath := strings.Join(kept, ";")
	cmdStr := fmt.Sprintf(
		`Set-ItemProperty -Path 'HKCU:\Environment' -Name 'PATH' -Value '%s' -Type ExpandString`,
		newPath,
	)
	if err := exec.Command("powershell", "-NoProfile", "-Command", cmdStr).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update user PATH: %v\n", err)
		return
	}
	fmt.Println("✓ Cleaned user PATH (registry)")
	// Broadcast so new terminals pick up the change.
	exec.Command("powershell", "-NoProfile", "-Command",
		`$signature = @'
[DllImport("user32.dll", SetLastError=true, CharSet=CharSet.Auto)]
public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam, uint fuFlags, uint uTimeout, out UIntPtr lpdwResult);
'@
	$null = Add-Type -MemberDefinition $signature -Name WinEnv -Namespace Win32 -PassThru
	$result = [UIntPtr]::Zero
	[Win32.WinEnv]::SendMessageTimeout([IntPtr]0xffff, 0x001A, [UIntPtr]::Zero, 'Environment', 2, 5000, [ref]$result) | Out-Null`,
	).Run() // best-effort
}
