#!/bin/bash

# Copyright Optimism PBC 2020
# MIT License
# github.com/ethereum-optimism

export DATA_TRANSPORT_LAYER__ADDRESS_MANAGER=`/opt/secret2env -name $SECRETNAME|grep -w DATA_TRANSPORT_LAYER__ADDRESS_MANAGER|sed 's/DATA_TRANSPORT_LAYER__ADDRESS_MANAGER=//g'`
export DATA_TRANSPORT_LAYER__L2_CHAIN_ID=`/opt/secret2env -name $SECRETNAME|grep -w DATA_TRANSPORT_LAYER__L2_CHAIN_ID|sed 's/DATA_TRANSPORT_LAYER__L2_CHAIN_ID=//g'`
export DATA_TRANSPORT_LAYER__CONFIRMATIONS=`/opt/secret2env -name $SECRETNAME|grep -w DATA_TRANSPORT_LAYER__CONFIRMATIONS|sed 's/DATA_TRANSPORT_LAYER__CONFIRMATIONS=//g'`
export DATA_TRANSPORT_LAYER__DANGEROUSLY_CATCH_ALL_ERRORS=`/opt/secret2env -name $SECRETNAME|grep -w DATA_TRANSPORT_LAYER__DANGEROUSLY_CATCH_ALL_ERRORS|sed 's/DATA_TRANSPORT_LAYER__DANGEROUSLY_CATCH_ALL_ERRORS=//g'`
export DATA_TRANSPORT_LAYER__DB_PATH=`/opt/secret2env -name $SECRETNAME|grep -w DATA_TRANSPORT_LAYER__DB_PATH|sed 's/DATA_TRANSPORT_LAYER__DB_PATH=//g'`
export DATA_TRANSPORT_LAYER__L1_RPC_ENDPOINT=`/opt/secret2env -name $SECRETNAME|grep -w DATA_TRANSPORT_LAYER__L1_RPC_ENDPOINT|sed 's/DATA_TRANSPORT_LAYER__L1_RPC_ENDPOINT=//g'`
export DATA_TRANSPORT_LAYER__LOGS_PER_POLLING_INTERVAL=`/opt/secret2env -name $SECRETNAME|grep -w DATA_TRANSPORT_LAYER__LOGS_PER_POLLING_INTERVAL|sed 's/DATA_TRANSPORT_LAYER__LOGS_PER_POLLING_INTERVAL=//g'`
export DATA_TRANSPORT_LAYER__POLLING_INTERVAL=`/opt/secret2env -name $SECRETNAME|grep -w DATA_TRANSPORT_LAYER__POLLING_INTERVAL|sed 's/DATA_TRANSPORT_LAYER__POLLING_INTERVAL=//g'`
export DATA_TRANSPORT_LAYER__SERVER_HOSTNAME=`/opt/secret2env -name $SECRETNAME|grep -w DATA_TRANSPORT_LAYER__SERVER_HOSTNAME|sed 's/DATA_TRANSPORT_LAYER__SERVER_HOSTNAME=//g'`
export DATA_TRANSPORT_LAYER__SYNC_FROM_L1=`/opt/secret2env -name $SECRETNAME|grep -w DATA_TRANSPORT_LAYER__SYNC_FROM_L1|sed 's/DATA_TRANSPORT_LAYER__SYNC_FROM_L1=//g'`
export DATA_TRANSPORT_LAYER__SYNC_FROM_L2=`/opt/secret2env -name $SECRETNAME|grep -w DATA_TRANSPORT_LAYER__SYNC_FROM_L2|sed 's/DATA_TRANSPORT_LAYER__SYNC_FROM_L2=//g'`
export DATA_TRANSPORT_LAYER__TRANSACTIONS_PER_POLLING_INTERVAL=`/opt/secret2env -name $SECRETNAME|grep -w DATA_TRANSPORT_LAYER__TRANSACTIONS_PER_POLLING_INTERVAL|sed 's/DATA_TRANSPORT_LAYER__TRANSACTIONS_PER_POLLING_INTERVAL=//g'`
export L1_NODE_WEB3_URL=`/opt/secret2env -name $SECRETNAME|grep -w L1_NODE_WEB3_URL|sed 's/L1_NODE_WEB3_URL=//g'`

rm -rf /db/LOCK
#!/bin/bash

set -e

RETRIES=${RETRIES:-60}

# go
exec node dist/src/services/run.js
