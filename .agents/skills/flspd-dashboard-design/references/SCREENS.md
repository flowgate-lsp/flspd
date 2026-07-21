# Dashboard Screen Specifications

This document defines the high-level technical specifications and data requirements for the `flspd` monitoring dashboard.

## Screen 1: Dashboard Overview (Home)
- **Status Indicators (Header)**:
    - `LndActive`: boolean (indicator: green icon if true, pulsating red if false)
    - `PostgresActive`: boolean (indicator: green if true)
    - `NostrReachable`: boolean (indicator: pulse indicator)
- **Key Metrics (Cards)**:
    - `TotalLiquidityLoki`: string (formatted from uint64)
    - `InboundAvailableLoki`: string (formatted from uint64)
    - `OutboundActiveLoki`: string (formatted from uint64)
    - `Revenue24hLoki`: string (total fees collected in last 24h)
- **Live Feed (Activity)**:
    - List of 10 most recent `lsps1` / `lsps2` events with timestamp and order state.
- **Top Peers (List)**:
    - Top 5 connected peer nodes by inbound/outbound liquidity volume.

---

## Screen 2: LSPS1 Orders (Management)
- **Table Data (Rows)**:
    - `OrderID`: string (UUID)
    - `PeerAlias`: string (fetched from channel graph)
    - `Status`: enum (CREATED, PAID, COMPLETED, FAILED)
    - `TotalAmountLoki`: string (client_balance + fee_total)
    - `CreatedAt`: ISO8601 (formatted to relative time)
- **Filtering Logic**:
    - Filter by: `Protocol` (LSPS1 vs LSPS2), `Status`, `AmountRange`.
    - Search by: `OrderID`, `PeerPubkey`.
- **Primary Actions**:
    - `Create Order` (manual order creation for testing)
    - `Sync Stale Orders` (triggers the recovery sweep implemented in the settlement service)

---

## Screen 3: Order Detail (Inspector)
- **Payment Lifecycle Block**:
    - `PaymentState`: string enum (EXPECT_PAYMENT, HOLD, PAID)
    - `PaymentMethod`: string (BOLT11, BOLT12, On-chain)
    - `Invoice`: string (selectable text / QR toggle)
- **Channel Opening Block**:
    - `FundingTxID`: string (clickable outpoint link)
    - `ChannelLocked`: boolean (state indicator)
    - `SCID`: string (Short Channel ID)
- **Log Inspector**:
    - Real-time rolling logs for the specific `OrderID`.
- **Secondary Actions**:
    - `Refund`: Trigger on-chain/off-chain refund for FAILED orders.
    - `Reopen`: Retry channel opening if order is stuck in PAID state.

---

## Screen 4: Liquidity Analytics (Metrics)
- **Charts**:
    - `CapacityChart`: Time-series of total inbound vs outbound (Area chart).
    - `PeerDistributionChart`: Bar chart of top 10 peers by capacity.
    - `OrderVolumeChart`: Daily counts of orders by protocol.
- **Exporting**:
    - `Download JSON`: Export raw metrics for external analysis.
