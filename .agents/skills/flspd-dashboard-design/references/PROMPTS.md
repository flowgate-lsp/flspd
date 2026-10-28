# Dashboard Design Prompts (for Product Designer Agent)

These prompts are designed to be used with a Product Designer Agent (e.g., Stitch) to elaborate the UI/UX for the Flowgate LSP Monitoring Dashboard.

## Design System Brief

**Prompt**:
> "Act as a Senior Lead Product Designer at a premium fintech agency. You are designing a monitoring dashboard for 'flspd', a high-performance Flokicoin Lightning Service Provider.
>
> **Aesthetic Requirements**:
> - **Theme**: Deep Dark Mode (e.g., #0a0a0c background).
> - **Style**: Glassmorphism with subtle frosted glass cards, thin borders (#ffffff1a), and deep shadows.
> - **Colors**: Harmonic neon accents. Use #00d2ff (electric blue) for primary actions, #00ff88 (mint) for success/completed states, #ff8c00 (orange) for pending/paid states, and #ff3b30 (red) for failures.
> - **Typography**: Inter or Outfit (Sans-serif). Clean, high-contrast, bold hierarchy for numerical metrics.
> - **Charts**: Minimalist line charts and circular progress indicators with glowing gradients.
>
> **Core Objective**: The user needs to feel absolute confidence in the system's performance and the security of their liquidity orders. The UI must be dense with data but clean and readable."

**Elevation system (2026-07 correction)**: an earlier implementation pass drifted from this brief — cards were built with ad-hoc `bg-black/20`–`bg-black/60` overlays and `border-white/5` (half the brief's specified `#ffffff1a`), which crushed every surface into one indistinguishable near-black. `web/src/index.css`'s `@theme` block now defines a real elevation scale instead:
- `--color-background` → `--color-surface-container-lowest` → `--color-surface-container-low` → `--color-surface` / `--color-surface-container` → `--color-surface-variant` → `--color-surface-container-high`, each a visible lightness step apart (not just alpha-blended black).
- `--color-outline` / `--color-outline-variant` replace raw `border-white/5` / `border-white/10` — use these tokens for card/divider borders instead of hand-rolled white-opacity values.
- Any new card should reach for `bg-surface-container-high`/`bg-surface-variant` (± opacity) rather than `bg-black/NN`; reserve real black (`bg-black/60`–`/80`) for modal scrims (`fixed inset-0` backdrops) only, where dimming the page behind a dialog is the correct behavior.

---

## Screen 1: The CommandCenter (Overview)

**Prompt**:
> "Generate a 'CommandCenter' Overview screen for flspd.
> **Header**: Real-time status indicators for Node (flnd), Database (Postgres), and Nostr Relay.
> **KPI Row**:
> 1. **Total Available Liquidity**: Large number in loki (e.g., 2.41 FLC) with a small 'change' percentage.
> 2. **Active Orders**: Current count of orders in PAID/PENDING state.
> 3. **Success Rate**: A circular progress gauge showing completed orders vs. failed.
> **Main Section**:
> - A 'Recent Events' feed showing live JSON-RPC activity.
> - A 'Peer Map' or list showing the top 5 most connected peers by liquidity volume.
> **Interactions**: Cards should have a subtle hover glowing effect."

---

## Screen 2: Order Matrix (Management)

**Prompt**:
> "Generate an 'Order Matrix' screen to manage LSPS1/LSPS2 orders.
> **List View**: A dense, but premium table showing Order ID (UUID), Peer Alias, Amount (loki), Fee, Payment State, and Created At.
> **States**:
> - `COMPLETED`: Soft green badge with 'Success' icon.
> - `PAID`: Soft blue badge with 'Settling' pulsing icon.
> - `FAILED`: Soft red badge with 'Alert' icon.
> **Filters**: A sidebar or top bar with quick toggles for 'LSPS1', 'LSPS2', 'Stuck Orders', and 'High Value'.
> **Search**: A glassmorphic search bar for Order IDs or Peer Pubkeys."

---

## Screen 3: Order DeepDive

**Prompt**:
> "Generate a 'Order DeepDive' detail screen.
> **Header**: Breadcrumbs (Orders > [UUID]) and a status stepper (Created -> Paid -> Funding -> Locked -> Completed).
> **Data Blocks**:
> - **Payment section**: QR code for BOLT11/12, amount details, and payment state with a live countdown if pending.
> - **Channel section**: Funding Transaction Outpoint (with link to explorer) and Short Channel ID (SCID).
> - **Log Viewer**: An embedded terminal-style window showing raw JSON-RPC logs for this specific order (e.g., `lsps1.get_order` responses).
> **Actions**: Buttons for 'Manual Recovery', 'Force Refund', and 'Notify Peer' (via Nostr)."

---

## Screen 4: Liquidity Radar (Metrics)

**Prompt**:
> "Generate a 'Liquidity Radar' analytics screen.
> **Visuals**:
> - A large area chart showing 'Inbound Capacity' vs 'Outbound Capacity' over the last 30 days.
> - A 'Peer Distribution' bar chart showing which nodes hold the most liquidity.
> - A 'Fee Revenue' chart showing loki earned from LSPS1 order fees.
> **Tooltips**: Hovering over charts should reveal detailed point data with high-contrast labels."
