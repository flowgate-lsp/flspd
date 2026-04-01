#!/bin/bash

# Stage all files related to the authentication, stability, and data mapping fixes
git add auth/nostr.go \
        cmd/flspd/run.go \
        common/opening_store.go \
        dashboard/service.go \
        lightning/client.go \
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
        frontend/src/pages/PeerNodes.tsx \
        frontend/src/pages/JITChannels.tsx \
        frontend/src/App.tsx \
        frontend/src/main.tsx \
        docker-compose.prod.yml

# Commit with a detailed message
git commit -m "fix(dashboard): unify data mapping, fix UI crashes and date parsing

- Standardize all Go structs (Peer, Summary, Order) with lowercase JSON tags
- Fix PeerNodes crash by referencing correct lowercase pub_key and address properties
- Resolve 'Invalid Date' issues by supporting ISO string parsing in all components
- Fix '0 loki' amount display by aligning frontend fields with backend tags
- Implement AuthProvider for reliable login redirects and shared session state
- Refactor WebSocket to a singleton context to prevent redundant connections
- Update JITChannels and OrderMatrix for full consistency across the dashboard"

echo "Stability and data mapping fixes successfully committed!"
