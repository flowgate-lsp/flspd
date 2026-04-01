#!/bin/bash

# Stage all files related to the authentication, stability, data mapping, and analytics fixes
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
git commit -m "feat(dashboard): real-time analytics, UI layout polish, and unified mapping

- Implement real-time Radar scoring (Liquidity, Uptime, Success) in backend
- Add 'get_analytics' RPC for real order distribution and 14-day revenue history
- Fix UI layout: Add proper padding and max-width to PeerNodes and JITChannels pages
- Standardize all Go structs (Peer, Summary, Order) with lowercase JSON tags
- Fix PeerNodes crash and 'Invalid Date' parsing across all components
- Refactor WebSocket to a singleton context and implement AuthProvider flow"

echo "Analytics, UI polish, and data mapping fixes successfully committed!"
