# op-run-block

This tool enables local op-geth EVM debugging,
to re-run bad blocks in a controlled local environment,
where arbitrary tracers can be attached,
and experimental changes can be tested quickly.

This helps debug why these blocks may fail or diverge in unexpected ways.
E.g. a block produced by op-reth that does not get accepted by op-geth
can be replayed in a debugger to find what is happening.

## Usage

```bash
go run . --rpc=http://my-debug-geth-endpoint:8545 --block=badblock.json --out=trace.txt
```

Where `badblock.json` looks like:

```json
[
  {
    "block": {
      "timestamp": ...
      "number": ...
      "transactions": [ ... ],
      ...
    }
  }
]
```

This type of block can be collected with:

```bash
cast rpc --rpc-url=http://localhost:8545 debug_getBadBlocks
```

See Go-ethereum `debug` RPC namespace docs: https://geth.ethereum.org/docs/interacting-with-geth/rpc/ns-debug
