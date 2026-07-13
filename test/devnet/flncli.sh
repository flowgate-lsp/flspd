#!/usr/bin/env bash
# Wrapper around flncli for a named devnet role, so callers don't have to
# repeat the macaroon/cert paths every time.
#
# Usage: ./flncli.sh <lsp|client|payer> <flncli args...>
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

ROLE="${1:?usage: flncli.sh <lsp|client|payer> <flncli args...>}"
shift

docker compose exec -T "flnd-${ROLE}" flncli \
  --network=regtest \
  --rpcserver=localhost:10009 \
  --macaroonpath=/root/.flnd/data/chain/flokicoin/regtest/admin.macaroon \
  --tlscertpath=/root/.flnd/tls.cert \
  "$@"
