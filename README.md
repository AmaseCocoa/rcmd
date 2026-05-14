# rcmd
> [!WARNING]
> **This software is NOT designed with security or adversarial tempering protection in mind.**
> 
> It is strictly intended for non-security purposes, such as a safety net to prevent human errors (e.g., accidentally executing deployment or destructive commands outside the designated working directories).


A minimal, low-profile command wrapper to restrict execution of specific commands and subcommands to designated directories.

## Features

- Restricts specific subcommands to specified directory trees (including symlink resolution).
- Auto-generates a documented configuration file on the first execution.
- **100% Vibe-Driven Development**: Quick and dirty code generated via vibe-coding because the author had absolutely zero desire to overthink this or waste precious time.

## Installation

### Prerequisites

- Go 1.26.3 or later

### Build from Source

```bash
git clone https://github.com/AmaseCocoa/rcmd.git
cd rcmd
go mod tidy
go build -ldflags="-s -w" -o rcmd main.go
sudo mv rcmd /usr/local/bin/
```

## Configuration

On the first run of `rcmd`, a default configuration file is automatically generated at:
`~/.config/rcmd/config.toml`

### Example `config.toml`

```toml
# Global configuration: Directory where the real binaries are hidden (Optional)
# bin_dir = "/usr/libexec/rcmd-targets"

[[commands]]
name = "git"
allowed_dir = "~/projects/secure-repo"
restricted_subcommands = ["push", "commit"]

[[commands]]
name = "docker"
allowed_dir = "~/docker-env"
restricted_subcommands = ["run", "rm"]
```

- `allowed_dir`: Supports both absolute paths and tilde (`~`) home directory expansion.
- `restricted_subcommands`: If left empty, all operations for that command will be blocked outside the specified directory.

## Usage

Prepend `rcmd` to the command you want to execute:

```bash
rcmd <command> [subcommand] [arguments...]
```

### Examples

When executed outside the allowed directory:
```bash
\(cd /tmp\) rcmd git push
rcmd: git push: permission denied in this directory
```

When executed within the allowed directory tree:
```bash
\(cd ~/projects/secure-repo/src\) rcmd git push
Everything up-to-date
```

Commands not listed in `config.toml` are transparently passed through without any restriction.

## LICENSE
MIT License