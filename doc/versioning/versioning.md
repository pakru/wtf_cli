# WTF CLI Versioning Guide

## Overview

WTF CLI uses **build-time variable injection** for versioning, following Go best practices. Version information is injected during compilation using `-ldflags`.

## Version Information

The version system tracks:
- **Version**: Semantic version (e.g., `v1.2.3`) derived from Git tags
- **Commit**: Short Git commit hash
- **Date**: Build timestamp in ISO 8601 format
- **Go Version**: Go compiler version used for the build

## Usage

### Check Version

```bash
# Any of these commands work:
wtf --version
wtf -v
wtf version
```

**Output:**
```
wtf version v1.2.3
  commit: abc1234
  built: 2025-10-02T23:43:09Z
  go: go1.24.3
```

## Building with Version Information

### Using Makefile (Recommended)

The Makefile automatically injects version information:

```bash
# Build with automatic version detection
make build

# Install with version info
make install

# Full installation
make install-full
```

The Makefile extracts version information from:
- **Git tags**: `git describe --tags --always --dirty`
- **Git commit**: `git rev-parse --short HEAD`
- **Build date**: Current UTC timestamp

### Manual Build

If building manually without make:

```bash
# Get version info
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "v0.0.0-dev")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)

# Build with version info
go build -ldflags="-X main.version=$VERSION -X main.commit=$COMMIT -X main.date=$DATE" -o wtf .
```

## Version Scheme

### Semantic Versioning

WTF CLI follows [Semantic Versioning 2.0.0](https://semver.org/):

```
vMAJOR.MINOR.PATCH[-PRERELEASE][+BUILD]
```

- **MAJOR**: Incompatible API changes
- **MINOR**: New functionality (backward compatible)
- **PATCH**: Bug fixes (backward compatible)
- **PRERELEASE**: Optional pre-release identifier (e.g., `-alpha`, `-beta`, `-rc.1`)
- **BUILD**: Optional build metadata (e.g., `+20250102`)

### Examples

```
v1.0.0          # Stable release
v1.2.3          # Patch update
v2.0.0-beta.1   # Pre-release
v1.0.0-dirty    # Uncommitted changes
abc1234         # No tags (commit hash only)
v0.0.0-dev      # Development build (no git)
```

## Creating Releases

### 1. Tag a Release

```bash
# Create annotated tag
git tag -a v1.0.0 -m "Release version 1.0.0"

# Push tag to remote
git push origin v1.0.0
```

### 2. Build Release Binary

```bash
# Build with version from tag
make build

# Verify version
./build/wtf --version
# Output: wtf version v1.0.0
```

### 3. Distribute

```bash
# Create release archive
tar -czf wtf-v1.0.0-linux-amd64.tar.gz -C build wtf

# Or use install script
make install-full
```

## Version Detection Logic

The Makefile uses this priority order:

1. **Git describe**: `git describe --tags --always --dirty`
   - Uses most recent tag + commits since tag
   - Example: `v1.2.3-5-gabc1234` (5 commits after v1.2.3)
   - Adds `-dirty` suffix if uncommitted changes exist

2. **Fallback**: `v0.0.0-dev`
   - Used when not in a git repository
   - Used when git is not available

## Development Builds

When building without tags or outside git:

```bash
# Development build shows:
wtf --version
# Output:
# wtf version dev
#   commit: none
#   built: unknown
#   go: go1.24.3
```

## Integration with CI/CD

### GitHub Actions Example

```yaml
name: Build Release

on:
  push:
    tags:
      - 'v*'

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
        with:
          fetch-depth: 0  # Fetch all history for git describe
      
      - uses: actions/setup-go@v4
        with:
          go-version: '1.24'
      
      - name: Build
        run: make build
      
      - name: Show version
        run: ./build/wtf --version
      
      - name: Create release
        uses: softprops/action-gh-release@v1
        with:
          files: build/wtf
```

## Best Practices

### For Developers

1. **Always use `make build`** instead of `go build` directly
2. **Tag releases** with semantic versions
3. **Test version output** before releasing
4. **Keep git history clean** for accurate version detection

### For Users

1. **Check version** after installation: `wtf --version`
2. **Report version** when filing bug reports
3. **Update regularly** to get latest fixes

### For Maintainers

1. **Follow semantic versioning** strictly
2. **Create annotated tags** with release notes
3. **Test builds** with and without git
4. **Document breaking changes** in MAJOR version updates

## Troubleshooting

### Version shows "dev"

**Cause**: Built without `-ldflags` or outside git repository

**Solution**: Use `make build` instead of `go build`

### Version shows "v0.0.0-dev"

**Cause**: No git tags exist in repository

**Solution**: Create first tag:
```bash
git tag -a v0.1.0 -m "Initial version"
```

### Version shows "-dirty"

**Cause**: Uncommitted changes in working directory

**Solution**: Commit or stash changes before building release

### Commit shows "unknown"

**Cause**: Not in a git repository

**Solution**: Clone from git or initialize git repository

## Implementation Details

### Code Structure

```
version.go          # Version variables and functions
main.go            # Version flag handling
Makefile           # Build-time version injection
```

### Key Variables

```go
var (
    version   = "dev"      // Set via -ldflags
    commit    = "none"     // Set via -ldflags
    date      = "unknown"  // Set via -ldflags
    goVersion = runtime.Version()  // Detected at runtime
)
```

### Build Command

```makefile
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)
go build -ldflags="$(LDFLAGS)" -o build/wtf .
```

## References

- [Semantic Versioning](https://semver.org/)
- [Go Build Constraints](https://pkg.go.dev/cmd/go#hdr-Build_constraints)
- [Git Describe](https://git-scm.com/docs/git-describe)
- [Go ldflags](https://pkg.go.dev/cmd/link)
