---
name: flspd-dashboard-design
description: UI/UX design and data-binding specs for the flspd operator dashboard (web/). Stack is React 19 + TanStack Router + Tailwind v4 + Recharts, built with Vite, deployed as a Cloudflare Workers static-assets app via wrangler — not Next.js/Vercel. Use whenever designing, building, or modifying a dashboard screen, or wiring a new data binding in web/.
---

# flspd Dashboard Design

## Stack facts

| | |
|---|---|
| UI framework | React 19 |
| Routing | `@tanstack/react-router` (not `react-router-dom`) |
| Styling | Tailwind v4 via `@tailwindcss/vite` |
| Charts | Recharts |
| Build | Vite |
| Deploy | `wrangler` → Cloudflare Workers static assets (`web/src/worker.ts` serves `web/dist/`) |

There is **no Next.js and no Vercel** anywhere in this deployment. Don't assume a Vercel/Next deploy model (Edge Functions, `next/image`, `next.config.js`-style conventions, etc.) applies here — it doesn't.

## Design prompts

High-fidelity design prompts for a Product Designer agent (aesthetic brief + per-screen prompts): [references/PROMPTS.md](references/PROMPTS.md).

## Screens & data bindings

Technical data-binding and screen requirements for each dashboard screen: [references/SCREENS.md](references/SCREENS.md).
