#!/bin/bash

# Stage all files related to the authentication, singleton WebSocket, currency renaming, and data mapping fixes
git add auth/nostr.go \
        cmd/flspd/run.go \
        common/opening_store.go \
        frontend/src/worker.ts \
        frontend/src/hooks/useAuth.ts \
        frontend/src/hooks/useEvents.ts \
        frontend/src/contexts/EventsContext.tsx \
        frontend/src/contexts/AuthContext.tsx \
        frontend/src/lib/api.ts \
        frontend/src/lib/utils.ts \
        frontend/src/pages/Login.tsx \
        frontend/src/pages/CommandCenter.tsx \
        frontend/src/pages/OrderMatrix.tsx \
        frontend/src/pages/OrderDetail.tsx \
        frontend/src/pages/Analytics.tsx \
        frontend/src/App.tsx \
        frontend/src/main.tsx \
        docker-compose.prod.yml

# Commit with a detailed message
git commit -m "fix(dashboard): resolve data mapping, auth state, and brand terminology

- Add JSON tags to Lsps1Order and fix field mapping (OrderId -> order_id, OrderTotalLoki -> amount)
- Implement AuthProvider and AuthContext for shared global authentication state
- Fix login redirect issue by lifting user state to context
- Rebrand currency units to FLC and loki across UI and backend
- Refactor WebSocket to a singleton context to fix redundant connections
- Harden NIP-98 auth for proxied environments and trailing slash consistency"

echo "Changes successfully committed! Run 'git push' to sync your changes."
