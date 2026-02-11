# Fix Mac CWD Bug (GitHub Issue #18)

## Problem

The `GetCwd()` function in `pkg/pty/pty.go` uses `/proc/<pid>/cwd`, which is Linux-specific and does not exist on macOS.

## Goals

1. Keep Linux behavior unchanged.
2. Add a native macOS CWD implementation.
3. Keep release automation reliable with macOS `CGO_ENABLED=1`.
4. Document an optional OSC 7-based path for future CWD tracking.

## Proposed Changes

Split `GetCwd` into platform-specific files and update release/build configuration. macOS builds will require CGO.

---

### PTY Package

#### [DELETE] `GetCwd` from `pkg/pty/pty.go`

Remove the existing Linux-specific method from `pty.go`.

---

#### [NEW] `pkg/pty/cwd_linux.go`

```go
//go:build linux

package pty

import (
	"fmt"
	"os"
)

// GetCwd returns the shell's current working directory
// by reading /proc/<pid>/cwd.
func (w *Wrapper) GetCwd() (string, error) {
	pid := w.GetPID()
	if pid == 0 {
		return "", fmt.Errorf("no process running")
	}

	procPath := fmt.Sprintf("/proc/%d/cwd", pid)
	cwd, err := os.Readlink(procPath)
	if err != nil {
		return "", fmt.Errorf("failed to read cwd: %w", err)
	}

	return cwd, nil
}
```

---

#### [NEW] `pkg/pty/cwd_darwin.go`

Use CGO with `proc_pidinfo(PROC_PIDVNODEPATHINFO)`.

```go
//go:build darwin && cgo

package pty

/*
#include <libproc.h>
#include <sys/proc_info.h>
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// GetCwd returns the shell's current working directory
// using libproc proc_pidinfo on macOS.
func (w *Wrapper) GetCwd() (string, error) {
	pid := w.GetPID()
	if pid == 0 {
		return "", fmt.Errorf("no process running")
	}

	var vpi C.struct_proc_vnodepathinfo
	const vpiSize = C.sizeof_struct_proc_vnodepathinfo

	ret, _ := C.proc_pidinfo(C.int(pid), C.PROC_PIDVNODEPATHINFO, 0, unsafe.Pointer(&vpi), vpiSize)
	if ret <= 0 {
		return "", fmt.Errorf("proc_pidinfo failed (ret=%d)", ret)
	}
	if ret != vpiSize {
		return "", fmt.Errorf("proc_pidinfo returned incomplete data: got %d bytes, expected %d", ret, vpiSize)
	}

	return C.GoString(&vpi.pvi_cdir.vip_path[0]), nil
}
```

---

### UI Package

#### [MODIFY] `pkg/ui/model.go`

Update the comment around directory polling to be OS-neutral:

```go
// Before:
// Update current directory from /proc/<pid>/cwd

// After:
// Update current directory from shell process
```

---

### Optional Future Enhancement: OSC 7 Shell Integration

This is not required for the immediate macOS fix, but it is a strong long-term option for CWD tracking.

#### What OSC 7 is

`OSC 7` is a shell-to-terminal escape sequence that reports the active working directory as a file URL:

```text
ESC ] 7 ; file://<host><absolute-path> ESC \\
```

Example:

```bash
printf "\033]7;file://%s%s\033\\" "$HOSTNAME" "$PWD"
```

#### Why this helps

1. Works without process introspection APIs.
2. Tracks prompt context directly (often more accurate for terminal UX).
3. Works well with remote sessions and terminal features when shell hooks are installed.

#### Tradeoffs

1. Requires shell integration scripts (`bash`/`zsh`/`fish`).
2. Needs OSC parsing in the PTY output path.
3. Must handle tmux passthrough and malformed sequences safely.

#### Suggested implementation path (separate phase)

1. Add optional OSC 7 parser in PTY output handling (`pkg/ui/terminal` or centralized PTY stream processing).
2. Add config toggle: `cwd_source = proc|osc7|auto`.
3. Add shell integration snippets under `docs/` and optionally installer hooks.
4. Keep current `GetCwd` as fallback when OSC 7 is unavailable.

---

### Build and Release Configuration

#### [ADD] `.goreleaser.linux.yml`

Linux-only artifacts (`CGO_ENABLED=0`):

```yaml
version: 2
project_name: wtf_cli

builds:
  - id: wtf_cli_linux
    main: ./cmd/wtf_cli
    binary: wtf_cli
    ldflags:
      - -s -w
      - -X wtf_cli/pkg/version.Version={{.Version}}
      - -X wtf_cli/pkg/version.Commit={{.ShortCommit}}
      - -X wtf_cli/pkg/version.Date={{.Date}}
    env:
      - CGO_ENABLED=0
    goos:
      - linux
    goarch:
      - amd64
      - arm64

archives:
  - id: wtf_cli_linux
    ids:
      - wtf_cli_linux
    formats:
      - tar.gz
    name_template: >-
      {{ .ProjectName }}_
      {{- .Version }}_
      {{- .Os }}_
      {{- .Arch }}
    files:
      - README.md
      - LICENSE

checksum:
  disable: true
```

#### [ADD] `.goreleaser.darwin.yml`

macOS-only artifacts (`CGO_ENABLED=1`):

```yaml
version: 2
project_name: wtf_cli

builds:
  - id: wtf_cli_darwin
    main: ./cmd/wtf_cli
    binary: wtf_cli
    ldflags:
      - -s -w
      - -X wtf_cli/pkg/version.Version={{.Version}}
      - -X wtf_cli/pkg/version.Commit={{.ShortCommit}}
      - -X wtf_cli/pkg/version.Date={{.Date}}
    env:
      - CGO_ENABLED=1
    goos:
      - darwin
    goarch:
      - arm64

archives:
  - id: wtf_cli_darwin
    ids:
      - wtf_cli_darwin
    formats:
      - tar.gz
    name_template: >-
      {{ .ProjectName }}_
      {{- .Version }}_
      {{- .Os }}_
      {{- .Arch }}
    files:
      - README.md
      - LICENSE

checksum:
  disable: true
```

#### [MODIFY] `.github/workflows/release.yml` (split release jobs)

Use separate build jobs per platform, then publish in a final job:

```yaml
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - run: make test

  build-linux:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: '~> v2'
          args: release --clean --skip=publish --config .goreleaser.linux.yml
      - uses: actions/upload-artifact@v4
        with:
          name: release-linux
          path: dist/*.tar.gz

  build-darwin:
    needs: test
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: '~> v2'
          args: release --clean --skip=publish --config .goreleaser.darwin.yml
      - uses: actions/upload-artifact@v4
        with:
          name: release-darwin
          path: dist/*.tar.gz

  publish:
    needs: [build-linux, build-darwin]
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - uses: actions/download-artifact@v4
        with:
          pattern: release-*
          path: dist
          merge-multiple: true
      - run: |
          cd dist
          sha256sum *.tar.gz > checksums.txt
      - run: |
          if gh release view "${GITHUB_REF_NAME}" >/dev/null 2>&1; then
            gh release upload "${GITHUB_REF_NAME}" dist/*.tar.gz dist/checksums.txt --clobber
          else
            gh release create "${GITHUB_REF_NAME}" dist/*.tar.gz dist/checksums.txt \
              --generate-notes --verify-tag
          fi
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

Notes:
1. This avoids darwin cgo cross-compilation on Ubuntu.
2. `goreleaser release --id` and split/merge flows are Pro-only.
3. `goreleaser build` does not create archives, so we use `release --skip=publish` per platform to produce `.tar.gz`.

---

## Verification Plan

### Automated

```bash
# Linux tests
cd /home/dev/project/wtf_cli/wtf_cli && go test ./...

# Linux release build target
cd /home/dev/project/wtf_cli/wtf_cli && CGO_ENABLED=0 go build ./...

# macOS build target (run on a Mac)
cd /home/dev/project/wtf_cli/wtf_cli && CGO_ENABLED=1 go build ./...
```

### Release Pipeline Dry Run

```bash
# Linux artifact
goreleaser release --snapshot --clean --skip=publish --config .goreleaser.linux.yml

# macOS artifact (run on macOS)
goreleaser release --snapshot --clean --skip=publish --config .goreleaser.darwin.yml
```

### Manual Verification (macOS)

1. Build with `CGO_ENABLED=1` on macOS.
2. Run `wtf_cli` and change directories with `cd`.
3. Confirm the UI status path updates correctly.

> [!IMPORTANT]
> macOS builds require `CGO_ENABLED=1` for `cwd_darwin.go`.
