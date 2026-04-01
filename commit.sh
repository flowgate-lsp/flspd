#!/bin/bash

# Stage all files related to the authentication and infrastructure hardening
git add auth/nostr.go \
        frontend/src/worker.ts \
        frontend/src/hooks/useAuth.ts \
        frontend/src/lib/api.ts \
        frontend/src/pages/Login.tsx \
        docker-compose.prod.yml

# Commit with a detailed message
git commit -m "feat(auth): enhance authentication security and reliability

- Fix trailing slash mismatch in NIP-98 URL reconstruction
- Implement robust query parameter parsing for proxied WebSockets
- Add backend verification during login to prevent unauthorized dashboard access
- Implement global 401/403 interceptor for automatic session termination
- Update Traefik labels to explicitly link multiple services on a single container"

echo "Changes successfully committed! Run 'git push' to sync your changes."
