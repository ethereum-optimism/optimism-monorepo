# Use a base image with necessary tools
FROM ubuntu:20.04

# Install required packages
RUN apt-get update && apt-get install -y \
    git \
    bash \
    curl \
    build-essential \
    jq \
    && rm -rf /var/lib/apt/lists/*

# Install Rust
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
ENV PATH="/root/.cargo/bin:${PATH}"

# Install just
RUN curl --proto '=https' --tlsv1.2 -sSf https://just.systems/install.sh | bash -s -- --to /usr/local/bin

# Install foundryup
RUN curl -L https://foundry.paradigm.xyz | bash
ENV PATH="/root/.foundry/bin:${PATH}"
RUN foundryup

# Set the working directory
WORKDIR /app

# Clone the repository
RUN git clone https://github.com/ethereum-optimism/optimism.git .

# Check out the target branch
RUN git checkout perm/op-contracts/v1.8.0

# Set the working directory to the root of the monorepo
WORKDIR /app

# Install correct foundry version
RUN just update-foundry

# Set the working directory to the root of the contracts package
WORKDIR /app/packages/contracts-bedrock

# Install dependencies
RUN just install

# Build the contracts package
RUN just forge-build

# Deliberately run the upgrade script with invalid args to trigger a build
RUN forge script ./scripts/upgrades/v1.3.0/v1.8.0-permissioned/DeployUpgrade.s.sol || true

# Set the working directory to where upgrade.sh is located
WORKDIR /app/packages/contracts-bedrock/scripts/upgrades/v1.3.0/v1.8.0-permissioned

# Set the entrypoint to the main.sh script
ENTRYPOINT ["./scripts/main.sh"]
