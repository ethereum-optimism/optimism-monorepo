# op-run-block

This tool enables local op-geth EVM debugging,
to re-run bad blocks in a controlled local environment,
where arbitrary tracers can be attached, and experimental changes can be tested quickly.

This helps debug why these blocks may fail or diverge in unexpected ways.
E.g. a block produced by op-reth that does not get accepted by op-geth
can be replayed in a debugger to find what is happening.

## Usage

```bash
go run . --rpc=http://my-debug-geth-endpoint:8545 --block=badblock.json
```
