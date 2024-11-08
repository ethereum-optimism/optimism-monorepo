## op-conductor runbook

### conductor configurations

In order to setup op-conductor, you need to configure the following env vars for both op-conductor and op-node service:

#### op-node

```env
OP_NODE_CONDUCTOR_ENABLED=true
OP_NODE_CONDUCTOR_RPC=<conductor-rpc-endpoint> # for example http://conductor:8545
```

#### op-conductor

```env
# prefix for the server id, used to identify the server in the raft cluster
RAFT_SERVER_ID_PREFIX=<prefix-for-server-id> # for example, sequencer-1, sequencer-2, etc
OP_CONDUCTOR_RAFT_STORAGE_DIR=<raft-storage-dir>
OP_CONDUCTOR_RPC_ADDR=<rpc-address> # for example, 0.0.0.0
OP_CONDUCTOR_RPC_PORT=<rpc-port> # for example, 8545
OP_CONDUCTOR_METRICS_ENABLED=true/false
OP_CONDUCTOR_METRICS_ADDR=<metrics-address> # for example 0.0.0.0
OP_CONDUCTOR_METRICS_PORT=<metrics-port> # for example 7300
OP_CONDUCTOR_CONSENSUS_PORT=<consensus-port> # for example 50050
OP_CONDUCTOR_PAUSED=true # set to true to start conductor in paused state
OP_CONDUCTOR_NODE_RPC=<node-rpc-endpoint> # for example, http://op-node:8545
OP_CONDUCTOR_EXECUTION_RPC=<execution-rpc-endpoint> # for example, http://op-geth:8545
OP_CONDUCTOR_NETWORK=<network-name> # for example, base-mainnet, op-mainnet, etc, should be same as OP_NODE_NETWORK
OP_CONDUCTOR_HEALTHCHECK_INTERVAL=<healthcheck-interval> # in seconds
OP_CONDUCTOR_HEALTHCHECK_UNSAFE_INTERVAL=<unsafe-interval> # Interval allowed between unsafe head and now measured in seconds in seconds
OP_CONDUCTOR_HEALTHCHECK_MIN_PEER_COUNT=<min-peer-count> # minimum number of peers required to be considered healthy
OP_CONDUCTOR_RAFT_BOOTSTRAP=true/false # set to true if you want to bootstrap the raft cluster
```

### How to bootstrap a sequencer cluster from scratch

In normal situations, you probably have a running sequencer already and you want to turn it into a HA cluster. What you need to do in this situation is to:

1. start a completely new sequencer with above mentioned configurations and
   1. `OP_CONDUCTOR_RAFT_BOOTSTRAP=true` set on op-conductor
   2. `OP_CONDUCTOR_PAUSED=true` set on op-conductor
   3. `OP_NODE_SEQUENCER_ENABLED=true` set on op-node
2. wait for the new sequencer to start and get synced up with the rest of the nodes
3. once the new sequencer is synced up, manually or use automation to stop sequencing on the old sequencer and start sequencing on the new sequencer
4. resume the conductor on the new sequencer by calling `conductor_resume` json rpc method on op-conductor
5. set `OP_CONDUCTOR_RAFT_BOOTSTRAP=false` on the sequencer so that it doesn't attempt to bootstrap the cluster during redeploy

Now you have a single HA sequencer which treats itself as the cluster leader! Next steps would be to add more sequencers to the cluster depending on your needs. For example, we want a 3-node cluster, you can follow the same process to add 2 more sequencers.

1. start a new sequencer with
   1. `OP_CONDUCTOR_RAFT_BOOTSTRAP=false` set on op-conductor
   2. `OP_CONDUCTOR_PAUSED=true` set on op-conductor
2. wait for the new sequencer to start and get synced up with the rest of the nodes
3. once the new sequencer is synced up, manually or use automation to add it to the cluster by calling `conductor_addServerAsVoter` json rpc method on the leader sequencer
4. call `conductor_clusterMembership` json rpc method on the leader sequencer to get the updated cluster membership
5. resume the conductor on the new sequencer by calling `conductor_resume` json rpc method on op-conductor

Once finished, you should have a 3-node HA sequencer cluster!

### Redeploy a HA sequencer

For every redeploy, depending on your underlying infrastructure, you need to make sure to:

1. `OP_CONDUCTOR_PAUSED=true` set on op-conductor so that conductor doesn't attempt to control sequencer while it's still syncing / redeploying
2. make sure sequencer is caught up with the rest of the nodes (this step isn't strictly necessary as conductor could handle this, but from a HA perspective, it does not make sense to have a sequencer that is lagging behind to join the cluster to potentially become the leader)
3. resume conductor after it's caught up with the rest of the nodes so that conductor can start managing the sequencer

### Disaster recovery

Whenever there are a disaster situation that you see no route to have 2 healthy conductor in the cluster communicating with each other, you need to manually intervene to resume sequencing. The steps are as follows:

1. call `conductor_pause` json rpc method on the all conductors so that they don't attempt to start / stop sequencer
2. choose a sequencer that can be used to resume sequencing
3. call `conductor_overrideLeader` json rpc method on the conductor to force it to treat itself as the leader
4. If no conductor is functioning, call `admin_overrideLeader` json rpc method on the op-node to force it to treat itself as the leader
5. manually start sequencing on the chosen sequencer
6. Go back to bootstrap step to re-bootstrap the cluster.
