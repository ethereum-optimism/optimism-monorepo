# `op-node`

Issues:
[monorepo](https://github.com/ethereum-optimism/optimism/issues?q=is%3Aissue%20state%3Aopen%20label%3AA-op-node)

Pull requests:
[monorepo](https://github.com/ethereum-optimism/optimism/pulls?q=is%3Aopen+is%3Apr+label%3AA-op-node)

User docs:
- [How to run a node](https://docs.optimism.io/builders/node-operators/rollup-node)

Specs:
- [rollup-node spec]

The op-node implements the [rollup-node spec].
It functions as a Consensus Layer client of an OP Stack chain.
This builds, relays and verifies the canonical chain of blocks.
The blocks are processed by an execution layer client, like [op-geth].

[rollup-node spec]: https://github.com/ethereum-optimism/specs/blob/main/specs/protocol/rollup-node.md
[op-geth]: https://github.com/ethereum-optimism/op-geth

## Quickstart

```bash
make op-node

# Network selection:
# - Join any of the pre-configured networks with the `--network` flag.
# - Alternatively, join a custom network with the `--rollup.config` flag.
#
# Essential Connections:
# - L1 ethereum RPC, to fetch blocks, receipts, finality
# - L1 beacon API, to fetch blobs
# - L2 engine API, to apply new blocks to
# - P2P TCP port, to expose publicly, to retrieve and relay the latest L2 blocks
# - P2P UDP port, to expose publicly, to discover other nodes to peer with
# - RPC port, to serve RPC of the op-node
#
# Other:
# - Sync mode: how to interact with the execution-engine,
#   such that it enters the preferred form of syncing:
#   - consensus-layer (block by block sync)
#   - execution-layer (e.g. snap-sync)
#
# Tip: every CLI flag has an env-var equivalent (run `op-node --help` for more information)
./bin/op-node \
  --network=op-sepolia \
  --l1=ws://localhost:8546 \
  --l1.beacon=http://localhost:4000 \
  --l2=ws://localhost:9001 \
  --p2p.listen.tcp=9222
  --p2p.listen.udp=9222
  --rpc.port=7000 \
  --syncmode=execution-layer

# If running inside docker, make sure to mount the below persistent data as (host) volume,
# it may be lost on restart otherwise:
# - P2P private key: auto-generated when missing, used to maintain a stable peer identity.
# - Peerstore DB: remember peer records to connect with, used to not wait for peer discovery.
# - Discovery DB: maintain DHT data, to avoid repeating some discovery work after restarting.
  --p2p.priv.path=opnode_p2p_priv.txt \
  --p2p.peerstore.path=opnode_peerstore_db \
  --p2p.discovery.path=opnode_discovery_db \
  --p2p.priv.path=opnode_p2p_priv.txt
```

## Usage

### Build from source

```bash
# from op-node dir:
make op-node
./bin/op-node --help
```

### Run from source

```bash
# from op-node dir:
go run ./cmd --help
```

### Build docker image

See `op-node` docker-bake target.

## Implementation overview

### Interactions

<!-- how this interacts with other modules -->
<!-- dependencies on other modules -->

## Product

The op-node **builds**, **relays** and **verifies** the canonical chain of blocks.

The op-node does not store critical data:
the op-node can recover from any existing L2 chain pre-state
that is sufficiently synced such that available input data can complete the sync.

The op-node **builds** blocks:
either from scratch as a sequencer, or from block-inputs (made available through L1) as verifier.

The block **relay** is a happy-path: the P2P sync is optional, and does not affect the ability to verify.
However, the block relay is still important for UX, as it lowers the latency to the latest state.

The blocks are **verified**: only valid L2 blocks that can be reproduced from L1 data are accepted.

### Optimization target

<!-- What do we optimize for in this implementation? -->

**Safely and reliably sync the canonical chain**

The op-node implements the three core product features as following:

- Block **building**: extend the chain at a throughput rate and latency that is safe to relay and verify.
- Block **relaying**: while keeping throughput high and latency low, prevent single points of failure.
- Block **verification**: efficiently sync, but always fully verify, follow the canonical chain.

Trade-offs are made here: verification safety is at odds ideal throughput, latency, efficiency.
Or in other words: safety vs. liveness. Chain parameters determine this.
The implementation offers this trade-off, siding with safety by default,
and design-choices should aim to improve the trade-off.

### Vision

The op-node is changing in two ways:
- [Reliability](#reliability): improve the reliability with improved processing, testing and syncing.
- [Interoperability](#interoperability): cross-chain messaging support.

#### Reliability

- Parallel derivation processes: [Issue 10864](https://github.com/ethereum-optimism/optimism/issues/10864)
- Event tests: [Issue 13163](https://github.com/ethereum-optimism/optimism/issues/13163)
- Improving P2P sync: [Issue 11779](https://github.com/ethereum-optimism/optimism/issues/11779)

#### Interoperability

The OP Stack is make chains natively interoperable:
messages between chains form safety dependencies, and verified asynchronously.
Asynchronous verification entails that the op-node reorgs away a block
if and when the block is determined to be invalid.

The [op-supervisor] specializes in this dependency verification work.

The op-node encapsulates all the single-chain concerns:
it prepares the local safety data-points (DA confirmation and block contents) for the op-supervisor.

The op-supervisor then verifies the cross-chain safety, and promotes the block safety level accordingly,
which the op-node then follows.

See [Interop specs] and [Interop design-docs] for more information about interoperability.

[op-supervisor]: ../op-supervisor/README.md

### User stories

<!-- As a **actor** I want **achievement** so that I **benefit** -->

As *a user* I want *reliability* so that I *don't miss blocks or fall out of sync*.
As *a RaaS dev* I want *easy configuration and monitoring* so that I *can run more chains*.
As *a customizoor* I want *clear extensible APIs* so that I *can avoid forking and be a contributor*.
As *a protocol dev* I want *integration with tests* so that I *assert protocol conformance*
As *a proof dev* I want *reusable state-transition code* so that I *don't reimplement the same thing*.

## Design principles

<!-- design choices / trade-offs -->
- Encapsulate the state-transition:
  - Use interfaces to abstract file-IO / concurrency / etc. away from state-transition logic.
  - Ensure code-sharing with action-tests and op-program.
- No critical database:
  - Persisting data is ok, but it should be recoverable from external data without too much work.
  - The best chain "sync" is no sync.
- Keep the tech-stack compatible with ethereum L1:
  - L1 offers well-adopted and battle tested libraries and standards, e.g. LibP2P, DiscV5, JSON-RPC.
  - L1 supports a tech-stack in different languages, ensuring client-diversity, important to L2 as well.
  - Downstream devs of OP-Stack should be able to pull in *one* instance of a library, that serves both OP-Stack and L1.

## Failure modes

This is a brief overview of what might fail, and how the op-node responds.

### L1 downtime

When the L1 data-source is temporarily unavailable the op-node `safe`/`finalized` progression halts.
Blocks may continue to sync through the happy-path if P2P connectivity is undisrupted.

### No batch confirmation

As per the [rollup-node spec] the sequencing-window ensures that after a bounded period of L1 blocks
the verifier will infer blocks, to ensure liveness of blocks with deposited transactions.
The op-node will continue to process the happy-path in the mean time,
which may have to be reorged out if it does not match the blocks that is inferred after sequencing window expiry.

### L1 reorg

L1 reorgs are detected passively during traversal: upon traversal to block `N+1`,
if the next canonical block has a parent-hash that does not match the
current block `N` we know the remote L1 chain view has diverged.

When this happens, the op-node assumes the local view is wrong, and resets itself to follow that of the remote node,
dropping any non-canonical blocks in the process.

### No L1 finality

When L1 does not finalize for an extended period of time,
the op-node is also unable to finalize the L2 chain for the same time.

Note that the `safe` block in the execution-layer is bootstrapped from the `finalized` block:
some verification work may repeat after a restart.

Blocks will continue to be derived from L1 batch-submissions, and optimistic processing will also continue to function.

### P2P failure

On P2P failure, e.g. issues with peering or failed propagation of block-data, the `unsafe` part of the chain may stall.
The `unsafe` part of the chain will no longer progress optimistically ahead of the `safe` part.

The `safe` blocks will continue to be derived from L1 however, providing a higher-latency access to the latest chain.

The op-node may pick back up the latest `unsafe` blocks after recovering its P2P connectivity,
and buffering `unsafe` blocks until the `safe` blocks progress meets the first known buffered `unsafe` block.

### Restarts and resyncing

After a restart, or detection of missing chain data,
the op-node dynamically determines what L1 data is required to continue, based on the syncing state of execution-engine.
If the sync-state is far behind, the op-node may need archived blob data to sync from the original L1 inputs.

A faster alternative may be to bootstrap through the execution-layer sync mode,
where the execution-engine may perform an optimized long-range sync, such as snap-sync.

## Testing

<!-- describe testing methods and approach to test coverage -->

- Unit tests: encapsulated functionality, fuzz tests, etc. in the op-node Go packages.
- `op-e2e` action tests: in-progress Go testing, focused on the onchain aspects,
  e.g. state-transition edge-cases. This applies primarily to the derivation pipeline.
- `op-e2e` system tests: in-process Go testing, focused on the offchain aspects of the op-node,
  e.g. background work, P2P integration, general service functionality.
- Local devnet tests: full end to end testing, but set up on minimal resources.
- Kurtosis tests: new automated devnet-like testing. Work in progress.
- Long-running devnet: roll-out for experimental features, to ensure sufficient stability for testnet users.
- Long-running testnet: battle-testing in public environment.
- Shadow-forks: design phase, testing experiments against shadow copies of real networks.

