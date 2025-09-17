# Frontend Dockerfile
FROM node:18-alpine AS base

# Install build dependencies and pnpm
RUN apk add --no-cache python3 make g++ && \
    npm install -g pnpm

# Set working directory
WORKDIR /app

# Copy package files
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/frontend/package.json ./apps/frontend/

# Install dependencies
RUN pnpm install --no-frozen-lockfile

# Copy source code
COPY apps/frontend ./apps/frontend

# Development stage
FROM base AS development
WORKDIR /app/apps/frontend
EXPOSE 3000
CMD ["pnpm", "dev"]

# Production stage
FROM base AS production
RUN pnpm --filter @apps/frontend build
EXPOSE 3000
CMD ["pnpm", "--filter", "@apps/frontend", "start"]
