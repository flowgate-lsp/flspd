#!/bin/bash

# Batch 1: Modernize Development Environment
git add justfile docker-compose.dev.yml Dockerfile.dev
git commit -m "build: modernize dev environment and standardize operations" \
           -m "- Standardized justfile commands (up, down, logs, etc.)
- Added healthchecks and grace periods to docker-compose.dev.yml
- Installed netcat in Dockerfile.dev for reliable healthchecks
- Resolved port conflict on host port 5173"

# Batch 2: Dashboard Backend & Authentication
git add auth/ dashboard/ cmd/flspd/dashboard_bridge.go
git commit -m "feat: implement dashboard backend with nip-98 authentication" \
           -m "- Added NIP-98 authentication logic for administrative access
- Implemented core dashboard service for metrics and config management
- Created bridge for real-time notification broadcasting to dashboard"

# Batch 3: Modernize Production Deployment
git add docker-compose.prod.yml
git commit -m "feat: modernize production deployment with traefik and healthchecks" \
           -m "- Configured traefik labels for lokihub and flspd
- Added healthchecks for postgres and flspd
- Optimized service dependencies and networking"

# Batch 4: Migrate Frontend to Cloudflare Workers
git add frontend/wrangler.jsonc frontend/src/worker.ts frontend/package.json frontend/.gitignore
git commit -m "feat: migrate frontend to cloudflare workers" \
           -m "- Added wrangler.jsonc for static asset configuration
- Implemented src/worker.ts for /api proxying and edge routing
- Added wrangler to frontend devDependencies
- Corrected frontend ignore patterns for cloudflare assets"

# Batch 5: Clean up and Automation
git add justfile docker-compose.prod.yml commit.sh
git commit -m "build: automate cloudflare deployment and cleanup docker stack" \
           -m "- Added deploy-frontend recipe to justfile
- Removed local frontend service from production docker-compose
- Added batch commit script for environment maintenance"
# Batch 6: Frontend Build Stabilization
git add frontend/src/pages/Analytics.tsx
git commit -m "fix: resolve frontend typescript build blockers" \
           -m "- Standardized Recharts tooltip formatter types in Analytics.tsx
- Ensured safe value casting to prevent TS2322 errors during CI"

echo "Batch commits completed successfully!"

