#!/bin/bash
set -e

# Batch 1: Refine Cloudflare Edge Proxy Configuration
git add frontend/wrangler.jsonc
git commit -m "build: refine upstream api endpoint" \
           -m "- Updated UPSTREAM_API to flspapi.flowgate.to for stable edge routing"

# Batch 2: Update Maintenance Scripts
git add commit.sh
git commit -m "build: update maintenance scripts for new deployment phase"

echo "Batch commits completed successfully!"
