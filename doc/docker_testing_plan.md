# Docker-Based Testing for `wtf_cli`

## Goals

- Ensure reliable, reproducible tests without modifying the host system.
- Test shell integration and CLI behavior in an isolated Ubuntu environment.
- Support automated local runs and CI.
- Remove any not required scripts any more which are related to tests.

## Scope

- Test pre-built `wtf` binary installation and shell integration in Docker.
- Validate shell integration (`scripts/integration.sh`) behavior with interactive bash.
- Exercise install/uninstall paths without touching the host.

## High-Level Architecture

- Host runs containers that simulate a user Linux environment.
- Base image: `ubuntu:24.04` (default) with bash, coreutils, curl, git, build-essential, network utils, development tools
- include build docker image into makefile
- Pre-built binary should be included in docker image, so install script can install it during image build
- Docker files organized in `docker/` folder.
- docker related files should be put into docker folder


## Components

- Dockerfile(s)
  - `docker/Dockerfile` for building test images.
- Compose file (optional, for convenience)
  - `docker/docker-compose.yml` running a one-off test service.
- Make targets
  - `make build` - Build the wtf binary first
  - `docker build -f docker/Dockerfile -t wtf-cli-test:latest .` - Build test image

## Image Contents (Dockerfile)

- Base: `ubuntu:24.04`
- Install packages:
  - `bash`, `ca-certificates`, `curl`, `git`, `make`, `gcc`, `g++`, `pkg-config`, `jq`, `bc`, `sudo`, `net-tools`, `iproute2`, `htop`, `btop`, `python3` 
- Create non-root user `tester` with passwordless sudo (safer than root).
- Set locale and PATH.
- Install wtf cli using install script into tester user home directory.


## Example Dockerfile (implemented)

```dockerfile
# syntax=docker/dockerfile:1
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash ca-certificates curl git make gcc g++ pkg-config jq bc sudo net-tools iproute2 htop btop python3 \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN useradd -ms /bin/bash tester && echo "tester ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/tester

# Copy project files including pre-built binary
COPY . /project
WORKDIR /project

# Install WTF CLI using pre-built binary
RUN chmod +x ./scripts/install.sh && ./scripts/install.sh

# Switch to tester user and home directory
USER tester
WORKDIR /home/tester

# Default entrypoint - drop into bash shell
ENTRYPOINT ["/bin/bash"]
```

## Example docker-compose.yml (optional)

```yaml
version: "3.9"
services:
  test:
    build:
      context: .
      dockerfile: docker/Dockerfile
    image: wtf-cli-test:latest
    environment:
      - WTF_DRY_RUN=true
    tty: true
    stdin_open: true
```

## Build Workflow

1. `make build` - Build the wtf binary
2. `docker build -f docker/Dockerfile -t wtf-cli-test:latest .` - Build Docker test image
3. `docker run --rm -it wtf-cli-test:latest` - Run interactive container for testing

## Risks and Mitigations

- Interactive shell behavior: use `bash -i -c` for hooks to trigger.
- TTY-dependent behaviors: run with `-t` when needed.
- Time-based races writing JSON: add small `sleep` before asserts.

## Acceptance Criteria

- Docker build workflow completes successfully and produces:
  - Built binary
  - creates docker image with installed wtf cli into this image for test purposes
  - `/home/tester/.wtf/last_command.json` and `/home/tester/.wtf/config.json` present with correct data
  - `/home/tester/.bashrc` contains `wtf` integration
  - `/home/tester/.local/bin/wtf` is present and executable
  - image is ready to be attached to bash and run wtf cli for test
- No host system changes occur.
