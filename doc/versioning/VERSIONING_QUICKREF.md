# WTF CLI Versioning - Quick Reference

## 🚀 Quick Start

### Check Version
```bash
wtf --version    # Preferred
wtf -v           # Short form
wtf version      # Subcommand
```

### Build with Version
```bash
make build       # Automatic version detection
```

### Create Release
```bash
git tag -a v1.0.0 -m "Release v1.0.0"
make build
./build/wtf --version
```

## 📋 Version Format

```
wtf version v1.2.3-dirty
  commit: abc1234
  built: 2025-10-03T06:50:49Z
  go: go1.25.1
```

## 🏷️ Version Suffixes

| Suffix | Meaning |
|--------|---------|
| `v1.0.0` | Clean release from tag |
| `v1.0.0-dirty` | Uncommitted changes |
| `v1.0.0-5-gabc1234` | 5 commits after v1.0.0 |
| `abc1234` | No tags (commit hash only) |
| `v0.0.0-dev` | No git repository |

## 🔧 Common Tasks

### Development Build
```bash
make build
./build/wtf --version
# Output: v0.0.1-dirty (if uncommitted changes)
```

### Release Build
```bash
git add .
git commit -m "Release prep"
git tag -a v1.0.0 -m "Release v1.0.0"
make build
./build/wtf --version
# Output: v1.0.0 (clean)
```

### Install with Version
```bash
make install-full
wtf --version
```

## 🧪 Testing

```bash
make test-version    # Test version functionality
make test            # All unit tests
make dev             # Full dev workflow
```

## 📦 Files

| File | Purpose |
|------|---------|
| `version.go` | Version variables and functions |
| `version_test.go` | Unit tests |
| `test_version.sh` | Integration tests |
| `doc/versioning.md` | Full documentation |

## 🎯 Semantic Versioning

```
vMAJOR.MINOR.PATCH
```

- **MAJOR**: Breaking changes
- **MINOR**: New features (backward compatible)
- **PATCH**: Bug fixes (backward compatible)

### Examples
- `v1.0.0` → `v1.0.1` - Bug fix
- `v1.0.0` → `v1.1.0` - New feature
- `v1.0.0` → `v2.0.0` - Breaking change

## 🔍 Troubleshooting

| Issue | Solution |
|-------|----------|
| Version shows "dev" | Use `make build` instead of `go build` |
| Version shows "v0.0.0-dev" | Create first tag: `git tag -a v0.1.0 -m "Initial"` |
| Version shows "-dirty" | Commit or stash changes |
| Commit shows "unknown" | Initialize git repository |

## 📚 More Info

See `doc/versioning.md` for complete documentation.
