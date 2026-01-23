# Specifications (BLIPs)

The `.agent/specs/` directory contains the preserved Bitcoin Lightning Improvement Proposals (BLIPs) relevant to this project, specifically the LSPS standards.

## Key LSPS Specs
- **BLIP-0050**: [LSPS0 - Transport & Versioning](specs/blip-0050.md). Defines how clients and LSPs communicate (JSON-RPC over Custom Lightning Messages).
- **BLIP-0051**: [LSPS1 - Channel Order Service](specs/blip-0051.md). Allows clients to purchase inbound liquidity (channels) from the LSP.
- **BLIP-0052**: [LSPS2 - JIT Channels](specs/blip-0052.md). Allows clients to receive payments without inbound capacity; LSP intercepts and opens channel.
- **BLIP-0055**: [LSPS5 - Webhook Registration](specs/blip-0055.md). Allows clients to register webhooks for push notifications (e.g. mobile wakeups).

## Other Specs
- [BLIP-0001](specs/blip-0001.md): General/Root spec.

These files have been copied here to ensure context preservation if the main `blips` directory is removed.
