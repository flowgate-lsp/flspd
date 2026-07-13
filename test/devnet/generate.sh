#!/usr/bin/env bash
# Mine N blocks on demand via lokid's `generate` RPC (no container
# restart involved — see the caveat in lokid.conf.sample about regtest
# wiping on restart). Coinbase rewards round-robin across whichever
# miningaddr lines are configured (see collect-addresses.sh).
#
# Usage: ./generate.sh <num_blocks>
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

N="${1:?usage: generate.sh <num_blocks>}"

docker compose exec -T chain lokid-cli \
  --regtest --notls --rpcuser=devnet --rpcpass=devnet --rpcserver=localhost:25216 \
  generate "$N"
