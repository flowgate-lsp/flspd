#!/usr/bin/env bash
# One-time (per session) bootstrap: fetches a receive address from each
# flnd role, writes them into data/chain/lokid.conf as miningaddr lines,
# then restarts the chain container ONCE to pick them up.
#
# Run this only right after bringing the devnet up for the first time in
# a session, before anything of value exists on the chain — lokid wipes
# its regtest database on every restart (see the note in
# lokid.conf.sample), so restarting later would erase mined funds.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

CONF=data/chain/lokid.conf

for role in lsp client payer; do
  echo "Fetching address for flnd-${role}..."
  ADDR=$(./flncli.sh "$role" newaddress p2wkh | python3 -c 'import json,sys; print(json.load(sys.stdin)["address"])')
  ROLE_UPPER=$(echo "$role" | tr '[:lower:]' '[:upper:]')
  sed -i "s|^; miningaddr=REPLACE_WITH_FLND_${ROLE_UPPER}_ADDRESS|miningaddr=${ADDR}|" "$CONF"
  echo "  flnd-${role}: ${ADDR}"
done

echo "Restarting chain to load mining addresses (regtest DB resets — nothing existed yet, so this is safe now)..."
docker compose restart chain
echo "Done. Fund nodes with: ./generate.sh <num_blocks>"
