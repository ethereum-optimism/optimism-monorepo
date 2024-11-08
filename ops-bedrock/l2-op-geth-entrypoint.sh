#!/bin/sh
set -exu

VERBOSITY=${GETH_VERBOSITY:-3}
GETH_DATA_DIR=/db
GETH_CHAINDATA_DIR="$GETH_DATA_DIR/geth/chaindata"
GENESIS_FILE_PATH="${GENESIS_FILE_PATH:-/genesis.json}"
CHAIN_ID=$(cat "$GENESIS_FILE_PATH" | jq -r .config.chainId)
RPC_PORT="${RPC_PORT:-8545}"
WS_PORT="${WS_PORT:-8546}"

if [ ! -d "$GETH_CHAINDATA_DIR" ]; then
	echo "$GETH_CHAINDATA_DIR missing, running init"
	echo "Initializing genesis."
	geth --verbosity="$VERBOSITY" init \
		--datadir="$GETH_DATA_DIR" \
		--state.scheme=hash \
		"$GENESIS_FILE_PATH"
else
	echo "$GETH_CHAINDATA_DIR exists."
fi

# Warning: Archive mode is required, otherwise old trie nodes will be
# pruned within minutes of starting the devnet.

exec geth \
	--datadir="$GETH_DATA_DIR" \
	--verbosity="$VERBOSITY" \
	--http \
	--http.corsdomain="*" \
	--http.vhosts="*" \
	--http.addr=0.0.0.0 \
	--http.port="$RPC_PORT" \
	--http.api=web3,debug,eth,txpool,net,engine,miner \
	--ws \
	--ws.addr=0.0.0.0 \
	--ws.port="$WS_PORT" \
	--ws.origins="*" \
	--ws.api=debug,eth,txpool,net,engine,miner \
	--syncmode=full \
	--nodiscover \
	--maxpeers=0 \
	--networkid="$CHAIN_ID" \
	--rpc.allow-unprotected-txs \
	--authrpc.addr="0.0.0.0" \
	--authrpc.port="8551" \
	--authrpc.vhosts="*" \
	--authrpc.jwtsecret=/config/jwt-secret.txt \
	--gcmode=archive \
  --state.scheme=hash \
	--metrics \
	--metrics.addr=0.0.0.0 \
	--metrics.port=6060 \
	"$@"
