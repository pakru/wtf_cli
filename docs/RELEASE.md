# Release Process

This document describes how to create a new release of WTF CLI.

## Overview

Releases are automated using GitHub Actions and GoReleaser. When you push a version tag, the system will:
1. Run all tests
2. Build Linux artifacts on Ubuntu
3. Build macOS artifacts on macOS
4. Create a GitHub Release and upload all artifacts

## Prerequisites

- Write access to the repository
- Git installed locally
- All tests passing on main branch

## Release Steps

### 1. Prepare the Release

Ensure you're on the latest main branch:
```bash
git checkout main
git pull origin main
```

Run tests to ensure everything works:
```bash
make check
```

### 2. Create and Push Tag

Version format: `vMAJOR.MINOR.PATCH` (e.g., `v1.2.3`)
- **MAJOR**: Breaking changes
- **MINOR**: New features, backward compatible
- **PATCH**: Bug fixes, backward compatible

```bash
# Create annotated tag
git tag -a v0.2.0 -m "Release v0.2.0"

# Push commit and tag
git push origin main
git push origin v0.2.0
```

**Important**: The tag is the source of truth for the version number.

### 3. Monitor Release Build

1. Go to: https://github.com/pakru/wtf_cli/actions
2. Find the "Release" workflow
3. Watch it build and publish the release
4. If it fails, check the logs and fix any issues

### 4. Verify Release

Once the workflow completes:

1. Check the [Releases page](https://github.com/pakru/wtf_cli/releases)
2. Verify the release is created with correct version
3. Download and test one of the binaries:
   ```bash
   # Download for your platform
   wget https://github.com/pakru/wtf_cli/releases/download/v0.2.0/wtf_cli_v0.2.0_linux_amd64.tar.gz
   tar -xzf wtf_cli_v0.2.0_linux_amd64.tar.gz
   ./wtf_cli --version
   ```

### 5. Update Release Notes (Optional)

If needed, edit the release notes on GitHub:
1. Go to the release page
2. Click "Edit release"
3. Add additional context, screenshots, or breaking changes
4. Save changes

## Testing Before Release

To test release artifact generation locally without publishing:

```bash
# Install GoReleaser (if not already installed)
go install github.com/goreleaser/goreleaser/v2@latest

# Linux artifacts
goreleaser release --snapshot --clean --skip=publish --config .goreleaser.linux.yml

# macOS artifacts (run this on macOS)
goreleaser release --snapshot --clean --skip=publish --config .goreleaser.darwin.yml

# Check the dist/ folder for built binaries
ls -la dist/
```

## Rollback

If you need to rollback a bad release:

### Delete the Tag

```bash
# Delete local tag
git tag -d v0.2.0

# Delete remote tag
git push origin :refs/tags/v0.2.0
```

### Delete the Release

1. Go to GitHub Releases page
2. Find the release
3. Click "Delete this release"

### Create New Fixed Release

Follow the release steps again with a new patch version (e.g., `v0.2.1`).

## Troubleshooting

### Release Workflow Fails

**Check the logs**: Go to Actions → Failed workflow → View logs

**Common issues**:
- Tests failing: Fix the tests and create a new tag
- Linux config error: Test with `goreleaser release --snapshot --skip=publish --config .goreleaser.linux.yml`
- macOS config error: Test with `goreleaser release --snapshot --skip=publish --config .goreleaser.darwin.yml` on macOS
- Missing permissions: Ensure GitHub Actions has write permissions

### Wrong Version in Binary

If the binary shows the wrong version:
- Ensure `version.txt` was committed before tagging
- Ensure the tag name matches the version in `version.txt`
- GoReleaser uses the tag name, not `version.txt`

### Binary Won't Run

Check for common issues:
- **Linux/macOS**: Make the binary executable with `chmod +x wtf_cli`
- **Wrong architecture**: Ensure you downloaded the correct binary for your platform
- **Dependencies**: Linux builds are non-cgo (`CGO_ENABLED=0`), while macOS builds use system libraries (`CGO_ENABLED=1`)

## Best Practices

### Semantic Versioning

Follow [Semantic Versioning](https://semver.org/):
- `v1.0.0` → `v1.0.1`: Bug fixes only
- `v1.0.0` → `v1.1.0`: New features, backward compatible
- `v1.0.0` → `v2.0.0`: Breaking changes

### Release Frequency

- **Patch releases**: Bug fixes can be released as needed
- **Minor releases**: Bundle features together for clarity
- **Major releases**: Plan carefully, document breaking changes

### Changelog

Commit messages are automatically included in the changelog. Use conventional commits:
- `feat: add new feature` → Listed under "Features"
- `fix: resolve bug` → Listed under "Bug Fixes"
- `perf: optimize performance` → Listed under "Performance Improvements"

### Pre-releases

For testing, create pre-release tags:
```bash
git tag v0.2.0-rc1
git push origin v0.2.0-rc1
```

Mark the release as "pre-release" in GitHub UI after it's created.
