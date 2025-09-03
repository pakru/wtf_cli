# syntax=docker/dockerfile:1
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash ca-certificates curl git make gcc g++ pkg-config jq bc sudo net-tools iproute2 htop btop python3 \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN useradd -ms /bin/bash tester && echo "tester ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/tester

# Copy project files
COPY . /project
WORKDIR /project

# Install WTF CLI using pre-built binary
RUN chmod +x ./scripts/install.sh && ./scripts/install.sh

# Switch to tester user and home directory
USER tester
WORKDIR /home/tester

# Default entrypoint - drop into bash shell
ENTRYPOINT ["/bin/bash"]