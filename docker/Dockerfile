# syntax=docker/dockerfile:1
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash ca-certificates curl git make gcc g++ pkg-config jq bc sudo mc net-tools iproute2 htop btop inetutils-ping inetutils-ping python3 \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN useradd -ms /bin/bash tester && echo "tester ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/tester

# Copy project files and build directory
COPY . /project
WORKDIR /project

# Switch to tester user and home directory first
USER tester
WORKDIR /home/tester

# Copy project to tester's home and install WTF CLI using pre-built binary
COPY --chown=tester:tester . /home/tester/project
WORKDIR /home/tester/project
RUN chmod +x ./scripts/install.sh && ./scripts/install.sh

# Default entrypoint - drop into bash shell
ENTRYPOINT ["/bin/bash"]