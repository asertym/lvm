package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"lvm/internal/manager"
	"lvm/internal/shim"
)

var (
	version = "0.1.0"
	mgr     *manager.Manager
)

func main() {
	home, err := lvmHome()
	if err != nil {
		fmt.Fprintf(os.Stderr, "lvm: cannot determine home directory: %v\n", err)
		os.Exit(1)
	}
	mgr = manager.New(home)

	root := &cobra.Command{
		Use:   "lvm",
		Short: "llama.cpp version manager",
		Long: `lvm manages multiple llama.cpp versions on your machine.
Install, switch, and update llama.cpp builds across stable and beta channels.
Run 'lvm init' once to set up your environment.`,
		SilenceUsage: true,
	}

	root.AddCommand(
		cmdInit(),
		cmdInstall(),
		cmdUse(),
		cmdCurrent(),
		cmdList(),
		cmdListRemote(),
		cmdUpdate(),
		cmdChannel(),
		cmdUninstall(),
		cmdVersion(),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// lvmHome returns the path to the lvm home directory.
// Uses LVM_HOME env var if set, otherwise ~/.lvm.
func lvmHome() (string, error) {
	if h := os.Getenv("LVM_HOME"); h != "" {
		return h, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".lvm"), nil
}

// cmdVersion prints the lvm version.
func cmdVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print lvm version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("lvm %s\n", version)
		},
	}
}

// cmdInit sets up lvm and automatically configures PATH.
// Zero friction: no prompts, no manual copy-paste required.
func cmdInit() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Set up lvm (run once after install)",
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

			fmt.Printf("\n%s lvm initialized at %s\n", green("✓"), mgr.Home())

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
				// Unix: append to the first available standard shell profile.
				home, _ := os.UserHomeDir()
				profiles := []string{".zshrc", ".bashrc", ".bash_profile", ".profile"}
				line := fmt.Sprintf("\n# lvm\nexport PATH=\"%s:$PATH\"\n", shimsDir)
				updated := false
				for _, p := range profiles {
					path := filepath.Join(home, p)
					if _, err := os.Stat(path); err == nil {
						data, _ := os.ReadFile(path)
						if strings.Contains(string(data), shimsDir) {
							fmt.Printf("%s PATH already configured in ~/%s\n", green("✓"), p)
						} else {
							f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
							if err != nil {
								return fmt.Errorf("could not update %s: %w", p, err)
							}
							f.WriteString(line)
							f.Close()
							fmt.Printf("%s Added to ~/%s\n", green("✓"), p)
						}
						updated = true
						break
					}
				}
				if !updated {
					fmt.Printf("%s No standard shell profile found. Please add PATH manually.\n", color.YellowString("⚠"))
				}
			}

			fmt.Printf("\n%s Restart your terminal to apply changes\n", bold("Next:"))
			fmt.Printf("Then run: %s\n\n", bold("lvm install latest"))
			return nil
		},
	}
}