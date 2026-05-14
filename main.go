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
# allowed_dirs = ["~/projects/secure-repo", "/var/www/html/prod-repo"]
# restricted_subcommands = ["push", "commit"]
`

type Config struct {
	BinDir   string       `toml:"bin_dir"`
	Commands []CmdRestric `toml:"commands"`
}

type CmdRestric struct {
	Name                  string   `toml:"name"`
	AllowedDirs           []string `toml:"allowed_dirs"`
	RestrictedSubcommands []string `toml:"restricted_subcommands"`
}

func loadConfig(configPath string) (Config, error) {
	var config Config
	_, err := toml.DecodeFile(configPath, &config)
	return config, err
}

func ensureConfig(configDir, configPath string) error {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return err
		}
		if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
			return err
		}
	}
	return nil
}

func runCommand(config Config, homeDir, targetCmd string, cmdArgs []string) {
	var targetConfig *CmdRestric
	for i, cmd := range config.Commands {
		if cmd.Name == targetCmd {
			targetConfig = &config.Commands[i]
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

			isAllowedLocation := false

			for _, allowedDir := range targetConfig.AllowedDirs {
				allowedPath := allowedDir
				if strings.HasPrefix(allowedPath, "~") {
					allowedPath = filepath.Join(homeDir, allowedPath[1:])
				}

				realAllowed, err := filepath.EvalSymlinks(allowedPath)
				if err != nil {
					realAllowed = filepath.Clean(allowedPath)
				}

				rel, err := filepath.Rel(realAllowed, realCurrent)
				if err == nil && !strings.HasPrefix(rel, "..") {
					isAllowedLocation = true
					break
				}
			}

			if !isAllowedLocation {
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

// detectShell returns the shell name (bash, zsh, fish) derived from $SHELL.
func detectShell() string {
	shell := os.Getenv("SHELL")
	base := filepath.Base(shell)
	switch base {
	case "bash", "zsh", "fish":
		return base
	default:
		return "bash"
	}
}

// shellSingleQuote wraps s in single quotes safe for POSIX shells by escaping
// any embedded single quotes as '\''.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// activateCommand prints alias definitions for all commands listed in config.toml.
// The output is intended to be eval'd by the user's shell:
//
//	eval "$(rcmd activate)"
func activateCommand(config Config, shell string) {
	rcmdPath, err := os.Executable()
	if err != nil {
		rcmdPath = "rcmd"
	}

	for _, cmd := range config.Commands {
		name := cmd.Name
		switch shell {
		case "fish":
			// fish uses a different quoting style; escape single quotes inside the name
			safeName := strings.ReplaceAll(name, "'", `\'`)
			safeRcmd := strings.ReplaceAll(rcmdPath, "'", `\'`)
			fmt.Printf("function %s; %s run %s $argv; end\n", safeName, safeRcmd, safeName)
		default: // bash, zsh and POSIX-compatible shells
			fmt.Printf("alias %s=%s\n", shellSingleQuote(name), shellSingleQuote(rcmdPath+" run "+name))
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: rcmd <subcommand> [args...]\n")
		fmt.Fprintf(os.Stderr, "\nSubcommands:\n")
		fmt.Fprintf(os.Stderr, "  run <command> [args...]   Run a command with rcmd restrictions\n")
		fmt.Fprintf(os.Stderr, "  activate [--shell <sh>]   Print shell aliases for commands in config.toml\n")
		os.Exit(1)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "rcmd: %v\n", err)
		os.Exit(1)
	}
	configDir := filepath.Join(homeDir, ".config", "rcmd")
	configPath := filepath.Join(configDir, "config.toml")

	if err := ensureConfig(configDir, configPath); err != nil {
		fmt.Fprintf(os.Stderr, "rcmd: %v\n", err)
		os.Exit(1)
	}

	config, err := loadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rcmd: failed to load config: %v\n", err)
		os.Exit(1)
	}

	subcommand := os.Args[1]

	switch subcommand {
	case "activate":
		shell := detectShell()
		args := os.Args[2:]
		for i := 0; i < len(args); i++ {
			if args[i] == "--shell" && i+1 < len(args) {
				shell = args[i+1]
				i++
			}
		}
		switch shell {
		case "bash", "zsh", "fish":
			// supported
		default:
			fmt.Fprintf(os.Stderr, "rcmd activate: unsupported shell %q (supported: bash, zsh, fish)\n", shell)
			os.Exit(1)
		}
		activateCommand(config, shell)

	case "run":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "usage: rcmd run <command> [args...]\n")
			os.Exit(1)
		}
		targetCmd := os.Args[2]
		cmdArgs := os.Args[3:]
		runCommand(config, homeDir, targetCmd, cmdArgs)

	default:
		fmt.Fprintf(os.Stderr, "rcmd: unknown subcommand %q\n", subcommand)
		fmt.Fprintf(os.Stderr, "usage: rcmd <subcommand> [args...]\n")
		fmt.Fprintf(os.Stderr, "\nSubcommands:\n")
		fmt.Fprintf(os.Stderr, "  run <command> [args...]   Run a command with rcmd restrictions\n")
		fmt.Fprintf(os.Stderr, "  activate [--shell <sh>]   Print shell aliases for commands in config.toml\n")
		os.Exit(1)
	}
}