#!/bin/bash

# Stage all files related to the authentication and singleton WebSocket refactor
git add auth/nostr.go \
        frontend/src/worker.ts \
        frontend/src/hooks/useAuth.ts \
        frontend/src/hooks/useEvents.ts \
        frontend/src/contexts/EventsContext.tsx \
        frontend/src/lib/api.ts \
        frontend/src/pages/Login.tsx \
        frontend/src/App.tsx \
        docker-compose.prod.yml

# Commit with a detailed message
git commit -m "feat(auth): singleton websocket and robust nip-98 verification

- Fix redundant WebSocket connections by implementing EventsProvider singleton context
- Resolve trailing slash mismatch in NIP-98 URL reconstruction
- Implement robust query parameter parsing for proxied connections
- Add backend verification during login with descriptive inline errors
- Implement global 401/403 logout interceptor
- Fix incorrect NIP-98 method tag name (method -> m)"

echo "Changes successfully committed! Run 'git push' to sync your changes."
