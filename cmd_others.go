package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"github.com/charmbracelet/huh"

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

// switchTo updates the active pointer and channel state, then prints confirmation.
func switchTo(id string, ch manager.Channel) error {
	if err := mgr.SetActive(id); err != nil {
		return fmt.Errorf("could not set active version: %w", err)
	}
	if err := mgr.SetChannelVersion(ch, id); err != nil {
		return fmt.Errorf("could not update channel: %w", err)
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
				// Check if any variant of this build is installed.
				versions, _ := mgr.ListInstalled()
				for _, v := range versions {
					if v.Build == r.TagName {
						installed = color.New(color.FgGreen).Sprint("  ✓ installed")
						break
					}
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
			_ = client.InvalidateCache() // force fresh check

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
			_, build := manager.ParseVersionID(active)
			_ = build

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

			// Re-use install logic by calling cobra directly would create circular
			// deps; instead we print the command for the user.
			bold := color.New(color.Bold).SprintFunc()
			fmt.Printf("\nRun: %s\n", bold(fmt.Sprintf("lvm install %s --backend %s --use", release.TagName, manifest.Backend)))
			return nil
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

	options := make([]huh.Option[string], 0, len(versions))
	for _, v := range versions {
		label := v.ID
		if v.ID == active {
			label = v.ID + " " + color.New(color.FgRed).Sprint("[active — cannot remove]")
		}
		options = append(options, huh.NewOption(label, v.ID))
	}

	if len(options) == 0 {
		return fmt.Errorf("no versions installed")
	}

	a := isatty.IsTerminal(os.Stdin.Fd())
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select a version to remove").
				Description("Cannot remove the active version. Arrow keys to navigate, Enter to confirm").
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

	if selectedID == active {
		return fmt.Errorf("cannot remove active version %q — run 'lvm use <other>' first", selectedID)
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
	green := color.New(color.FgGreen, color.Bold).SprintFunc()
	fmt.Printf("%s Removed %s\n", green("✓"), id)
	return nil
}

func valueOrNone(s string) string {
	if s == "" {
		return color.New(color.Faint).Sprint("(none)")
	}
	return s
}
