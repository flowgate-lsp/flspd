# Specifications (bLIPs)

This directory holds the LSPS bLIP spec text referenced by the `flspd-lsps-protocol` skill — the canonical upstream Lightning Service Provider Standards, verbatim (their units/terminology are the real Lightning Network's sat/msat, which is correct for these spec texts even though flspd itself runs on Flokicoin loki/mloki — see the `flspd-lsps-protocol` skill's units note).

## Key LSPS Specs
- **bLIP-0050**: [LSPS0 - Transport & Versioning](blip-0050.md). Defines how clients and LSPs communicate (JSON-RPC over Custom Lightning Messages).
- **bLIP-0051**: [LSPS1 - Channel Order Service](blip-0051.md). Allows clients to purchase inbound liquidity (channels) from the LSP.
- **bLIP-0052**: [LSPS2 - JIT Channels](blip-0052.md). Allows clients to receive payments without inbound capacity; LSP intercepts and opens channel.
- **bLIP-0055**: [LSPS5 - Webhook Registration](blip-0055.md). Allows clients to register webhooks for push notifications (e.g. mobile wakeups).

## Other Specs
- [bLIP-0001](blip-0001.md): the bLIP process itself (general/root spec).
