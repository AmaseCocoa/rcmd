package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

const defaultConfig = `# rcmd configuration file

# Global configuration: Directory where the real binaries are hidden (Optional)
# If empty, rcmd will search from the system PATH.
# bin_dir = "/usr/libexec/rcmd-targets"

# [[commands]]
# name = "git"
# allowed_dir = "~/projects/secure-repo"
# restricted_subcommands = ["push", "commit"]
`


type Config struct {
	BinDir   string       `toml:"bin_dir"`
	Commands []CmdRestric `toml:"commands"`
}

type CmdRestric struct {
	Name                  string   `toml:"name"`
	AllowedDir            string   `toml:"allowed_dir"`
	RestrictedSubcommands []string `toml:"restricted_subcommands"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: rcmd <command> [subcommand] [args...]\n")
		os.Exit(1)
	}

	targetCmd := os.Args[1]
	cmdArgs := os.Args[2:]

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "rcmd: %v\n", err)
		os.Exit(1)
	}
	configDir := filepath.Join(homeDir, ".config", "rcmd")
	configPath := filepath.Join(configDir, "config.toml")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := os.MkdirAll(configDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "rcmd: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "rcmd: %v\n", err)
			os.Exit(1)
		}
	}

	var config Config
	if _, err := toml.DecodeFile(configPath, &config); err != nil {
		fmt.Fprintf(os.Stderr, "rcmd: failed to load config: %v\n", err)
		os.Exit(1)
	}

	var targetConfig *CmdRestric
	for _, cmd := range config.Commands {
		if cmd.Name == targetCmd {
			targetConfig = &cmd
			break
		}
	}

	if targetConfig != nil {
		isRestricted := false
		if len(cmdArgs) > 0 {
			isRestricted = slices.Contains(targetConfig.RestrictedSubcommands, cmdArgs[0])
		} else if len(targetConfig.RestrictedSubcommands) == 0 {
			isRestricted = true
		}

		if isRestricted {
			currentDir, err := os.Getwd()
			if err != nil {
				fmt.Fprintf(os.Stderr, "rcmd: %v\n", err)
				os.Exit(1)
			}

			realCurrent, err := filepath.EvalSymlinks(currentDir)
			if err != nil {
				realCurrent = filepath.Clean(currentDir)
			}
			
			allowedPath := targetConfig.AllowedDir
			if strings.HasPrefix(allowedPath, "~") {
				allowedPath = filepath.Join(homeDir, allowedPath[1:])
			}
			
			realAllowed, err := filepath.EvalSymlinks(allowedPath)
			if err != nil {
				realAllowed = filepath.Clean(allowedPath)
			}

			rel, err := filepath.Rel(realAllowed, realCurrent)
			if err != nil || strings.HasPrefix(rel, "..") {
				fmt.Fprintf(os.Stderr, "rcmd: %s %s: permission denied in this directory\n", targetCmd, strings.Join(cmdArgs, " "))
				os.Exit(1)
			}
		}
	}

	execName := targetCmd
	if config.BinDir != "" {
		execName = filepath.Join(config.BinDir, targetCmd)
	}

	cmd := exec.Command(execName, cmdArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			os.Exit(exitError.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "%s: %v\n", targetCmd, err)
		os.Exit(1)
	}
}