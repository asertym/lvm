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

	"llava/internal/manager"
	"llava/internal/selfupdate"
	"llava/internal/shim"
	"llava/internal/updater"
)

var (
	version    = "0.2.3"
	mgr        *manager.Manager
	skipUpdate bool
)

func main() {
	home, err := llavaHome()
	if err != nil {
		fmt.Fprintf(os.Stderr, "llava: cannot determine home directory: %v\n", err)
		os.Exit(1)
	}
	mgr = manager.New(home)

	root := &cobra.Command{
		Use:   "llava",
		Short: "llama.cpp version manager",
		Long: `llava manages multiple llama.cpp versions on your machine.
Install, switch, and update llama.cpp builds across stable and beta channels.
Run 'llava init' once to set up your environment.`,
		SilenceUsage: true,
	}

	root.PersistentFlags().BoolVar(&skipUpdate, "skip-update-check", false, "Skip the update check")

	root.AddCommand(
		cmdInit(),
		cmdInstall(),
		cmdUse(),
		cmdCurrent(),
		cmdList(),
		cmdListRemote(),
		cmdUpdate(),
		cmdChannel(),
		cmdFetch(),
		cmdUninstall(),
		cmdUninstallSelf(),
		cmdVersion(),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// llavaHome returns the path to the llava home directory.
// Uses LLAVA_HOME env var if set, otherwise ~/.llava.
func llavaHome() (string, error) {
	if h := os.Getenv("LLAVA_HOME"); h != "" {
		return h, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".llava"), nil
}

// cmdVersion prints the llava version and checks for updates.
func cmdVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print llava version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("llava %s\n", version)

			if skipUpdate {
				return
			}

			latest, err := updater.LatestReleaseWithAssets()
			if err != nil {
				// Network error — silently skip.
				return
			}

			if !updater.SemverLess(version, latest.TagName) {
				fmt.Printf("  (latest)\n")
				return
			}

			if !isatty.IsTerminal(os.Stdin.Fd()) {
				// Non-interactive: just inform.
				yellow := color.New(color.FgYellow).SprintFunc()
				bold := color.New(color.Bold).SprintFunc()
				fmt.Printf("\n%s %s → %s available!\n", yellow("Update:"), bold(version), latest.TagName)
				fmt.Printf("  Run 'llava version' again to install.\n")
				return
			}

			var choice string
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title("Update llava?").
						Description("Download and install llava").
						Options(
							huh.NewOption("update", "update"),
							huh.NewOption("skip for now", "skip"),
						).Value(&choice),
				),
			)

			if err := form.Run(); err != nil {
				if err == huh.ErrUserAborted {
					return
				}
				fmt.Fprintf(os.Stderr, "update prompt failed: %v\n", err)
				return
			}

			if choice != "update" {
				return
			}

			// Self-update.
			asset, err := updater.AssetForPlatform(latest)
			if err != nil {
				fmt.Fprintf(os.Stderr, "could not find binary for this platform: %v\n", err)
				return
			}

			fmt.Printf("Downloading %s...\n", asset.Name)
			if err := selfupdate.Update(asset.URL, ""); err != nil {
				fmt.Fprintf(os.Stderr, "update failed: %v\n", err)
				os.Exit(1)
			}

			green := color.New(color.FgGreen, color.Bold).SprintFunc()
			fmt.Printf("\n%s Updated to %s. Restart llava to use the new version.\n", green("✓"), latest.TagName)
		},
	}
}

// cmdInit sets up llava and automatically configures PATH.
// Zero friction: no prompts, no manual copy-paste required.
func cmdInit() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Set up llava (run once after install)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := mgr.Init(); err != nil {
				return err
			}

			shimMgr := shim.NewManager(mgr.ShimsDir(), mgr.Home())
			if err := shimMgr.EnsureAll(); err != nil {
				return fmt.Errorf("shim creation failed: %w", err)
			}

			green := color.New(color.FgGreen, color.Bold).SprintFunc()
			bold := color.New(color.Bold).SprintFunc()

			fmt.Printf("\n%s llava initialized at %s\n", green("✓"), mgr.Home())

			shimsDir := mgr.ShimsDir()

			if runtime.GOOS == "windows" {
				// Write shims dir to user PATH as REG_EXPAND_SZ via PowerShell.
				// REG_EXPAND_SZ ensures the value survives new terminal sessions
				// and correctly merges with the system PATH — REG_SZ does not.
				psWrite := fmt.Sprintf(
					`$current = [Environment]::GetEnvironmentVariable('PATH', 'User');
					 if ($null -eq $current) { $current = '' };
					 $parts = $current -split ';' | Where-Object { $_ -ne '' };
					 if ($parts -notcontains '%s') {
					   $new = ('%s;' + ($parts -join ';')).TrimEnd(';');
					   Set-ItemProperty -Path 'HKCU:\Environment' -Name 'PATH' -Value $new -Type ExpandString
					 }`,
					shimsDir, shimsDir,
				)
				if err := exec.Command("powershell", "-NoProfile", "-Command", psWrite).Run(); err != nil {
					return fmt.Errorf("failed to update Windows PATH: %w", err)
				}
			} else {
				// Unix: append to ALL existing standard shell profiles (not just the first).
				home, _ := os.UserHomeDir()
				profiles := []string{".zshrc", ".bashrc", ".bash_profile", ".profile"}
				line := fmt.Sprintf("\n# llava\nexport PATH=\"%s:$PATH\"\n", shimsDir)
				for _, p := range profiles {
					profilePath := filepath.Join(home, p)
					if _, err := os.Stat(profilePath); err != nil {
						continue
					}
					data, _ := os.ReadFile(profilePath)
					if strings.Contains(string(data), shimsDir) {
						fmt.Printf("%s PATH already configured in ~/%s\n", green("✓"), p)
					} else {
						f, err := os.OpenFile(profilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
						if err != nil {
							fmt.Fprintf(os.Stderr, "warning: could not update ~/%s: %v\n", p, err)
							continue
						}
						f.WriteString(line)
						f.Close()
						fmt.Printf("%s Added to ~/%s\n", green("✓"), p)
					}
				}
				// Check if at least one profile was found.
				foundAny := false
				for _, p := range profiles {
					if _, err := os.Stat(filepath.Join(home, p)); err == nil {
						foundAny = true
						break
					}
				}
				if !foundAny {
					fmt.Printf("%s No standard shell profile found. Please add PATH manually.\n", color.YellowString("⚠"))
				}
			}

			fmt.Printf("\n%s Restart your terminal to apply changes\n", bold("Next:"))
			fmt.Printf("Then run: %s\n\n", bold("llava install latest"))
			return nil
		},
	}
}
