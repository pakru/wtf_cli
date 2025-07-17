# WTF CLI Scripts

This directory contains installation and testing scripts for the WTF CLI project.

## Scripts Overview

### Installation Scripts

#### `integration.sh`
The shell integration script that captures command information in real-time.

**Usage:**
```bash
# Manual installation
cp scripts/integration.sh ~/.wtf/integration.sh
echo "source ~/.wtf/integration.sh" >> ~/.bashrc
source ~/.bashrc
```

**What it does:**
- Captures every command executed in bash
- Records command text, exit code, timing, and working directory
- Stores data in JSON format at `~/.wtf/last_command.json`
- Uses bash hooks (PROMPT_COMMAND, trap DEBUG/ERR/EXIT) for real-time capture
- Handles command output capture via temporary files

#### `install.sh`
The main installation script that automates the complete setup of WTF CLI.

**Usage:**
```bash
# Install WTF CLI
./scripts/install.sh

# Uninstall WTF CLI
./scripts/install.sh uninstall

# Show help
./scripts/install.sh help
```

**What it does:**
- Builds the WTF CLI binary from source
- Installs the binary to `~/.local/bin/wtf`
- Sets up shell integration in `~/.bashrc`
- Creates default configuration at `~/.wtf/config.json`
- Adds `~/.local/bin` to PATH if needed
- Creates backups before modifying system files

**Uninstallation:**
- Removes the WTF CLI binary
- Removes the `~/.wtf` directory (with confirmation)
- Removes shell integration from `~/.bashrc`
- Restores original bash configuration

### Testing Scripts

#### `test_installation.sh`
Comprehensive test suite for the installation process.

**Usage:**
```bash
# Test the installation script
./scripts/test_installation.sh

# Or via make
make test-install
```

**What it tests:**
- Installation script completes without errors
- Binary is installed correctly
- Shell integration is set up properly
- Configuration files are created
- Binary works in dry-run mode
- Uninstallation removes everything cleanly

#### `test_shell_integration_e2e.sh`
End-to-end test for shell integration functionality.

**Usage:**
```bash
# Test shell integration
./scripts/test_shell_integration_e2e.sh

# Or via make
make test-shell-e2e
```

**What it tests:**
- Shell integration script captures commands correctly
- JSON files are created with proper format
- Go CLI can read shell integration data
- Command timing and exit codes are accurate
- Integration works with the WTF CLI binary

## Running from Different Locations

All scripts are designed to work when run from either:
- The project root directory: `./scripts/script_name.sh`
- The scripts directory: `./script_name.sh`

The scripts automatically detect their location and adjust paths accordingly.

## Integration with Makefile

These scripts are integrated with the project Makefile:

```bash
make install-full    # Run ./scripts/install.sh
make uninstall       # Run ./scripts/install.sh uninstall
make test-install    # Run ./scripts/test_installation.sh
make test-shell-e2e  # Run ./scripts/test_shell_integration_e2e.sh
```

## Safety Features

- **Backup Creation**: Installation script backs up `.bashrc` before modification
- **Error Handling**: All scripts include comprehensive error checking
- **Confirmation Prompts**: Destructive operations require user confirmation
- **Test Environment**: Test scripts use isolated temporary environments
- **Graceful Cleanup**: All scripts clean up after themselves on exit
