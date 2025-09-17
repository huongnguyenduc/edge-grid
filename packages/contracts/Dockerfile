# Contracts Dockerfile
FROM ghcr.io/foundry-rs/foundry:latest

# Set working directory
WORKDIR /app

# Copy source code
COPY packages/contracts ./

# Initialize git repository (needed for forge install)
RUN git init

# Install dependencies
RUN forge install

# Build contracts
RUN forge build

# Default command
CMD ["forge", "test", "-vvv"]
