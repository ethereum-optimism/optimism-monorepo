<!--![cannon](https://upload.wikimedia.org/wikipedia/commons/8/80/Cannon%2C_Château_du_Haut-Koenigsbourg%2C_France.jpg)-->
<!--![cannon](https://cdn1.epicgames.com/ue/product/Featured/SCIFIWEAPONBUNDLE_featured-894x488-83fbc936b6d86edcbbe892b1a6780224.png)-->
<!--![cannon](https://static.wikia.nocookie.net/ageofempires/images/8/80/Bombard_cannon_aoe2DE.png/revision/latest/top-crop/width/360/height/360?cb=20200331021834)-->
<!--![cannon](https://paradacreativa.es/wp-content/uploads/2021/05/Canon-orbital-GTA-01.jpg)-->

---

Cannon *(cannon cannon cannon)* is an onchain MIPS instruction emulator.
Cannon supports EVM-equivalent fault proofs by enabling Geth to run onchain,
one instruction at a time, as part of an interactive dispute game.

* It's Go code
* ...that runs an EVM
* ...emulating a MIPS machine
* ...running compiled Go code
* ...that runs an EVM

For more information, see [Docs](./docs/README.md).

## Usage

```shell
# Build op-program server-mode and MIPS-client binaries.
cd ../op-program
make op-program # build

# Switch back to cannon, and build the CLI
cd ../cannon
make cannon

# Transform MIPS op-program client binary into first VM state.
# This outputs state.bin.gz (VM state) and meta.json (for debug symbols).
./bin/cannon load-elf --type singlethreaded-2 --path=../op-program/bin/op-program-client.elf

# Run cannon emulator (with example inputs)
# Note that the server-mode op-program command is passed into cannon (after the --),
# it runs as sub-process to provide the pre-image data.
#
# Note:
#  - The L2 RPC is an archive L2 node on OP MAINNET.
#  - The L1 RPC is a non-archive RPC, also change `--l1.rpckind` to reflect the correct L1 RPC type.
#  - The network flag is only suitable for specific networks(https://github.com/ethereum-optimism/superchain-registry/blob/main/chainList.json). If you are running on the devnet, please use '--l2.genesis' to supply a path to the L2 devnet genesis file.
./bin/cannon run \
    --pprof.cpu \
    --info-at '%10000000' \
    --proof-at '=<TRACE_INDEX>' \
    --stop-at '=<STOP_INDEX>' \
    --snapshot-at '%1000000000' \
    --input ./state.bin.gz \
    -- \
    ../op-program/bin/op-program \
    --network <network name> \
    --l1 <L1_URL> \
    --l2 <L2_URL> \
    --l1.head <L1_HEAD> \
    --l2.claim <L2_CLAIM> \
    --l2.head <L2_HEAD> \
    --l2.blocknumber <L2_BLOCK_NUMBER> \
    --l2.outputroot <L2_OUTPUT_ROOT>
    --datadir /tmp/fpp-database \
    --log.format terminal \
    --server

# Add --proof-at '=12345' (or pick other pattern, see --help)
# to pick a step to build a proof for (e.g. exact step, every N steps, etc.)

# Also see `./bin/cannon run --help` for more options
```

## Contracts

The Cannon contracts:
- `MIPS.sol`: A MIPS emulator implementation, to run a single instruction onchain, with merkleized VM memory.
- `PreimageOracle.sol`: implements the pre-image oracle ABI, to support the instruction execution pre-image requests.

The smart-contracts are integrated into the Optimism monorepo contracts:
[`../packages/contracts-bedrock/src/cannon`](../packages/contracts-bedrock/src/cannon)

## `mipsevm`

`mipsevm` is Go tooling to test the onchain MIPS implementation, and generate proof data.

## `example`

Example programs that can be run and proven with Cannon.
Optional dependency, but required for `mipsevm` Go tests.
See [`testdata/example/Makefile`](./testdata/example/Makefile) for building the example MIPS binaries.

## License

MIT, see [`LICENSE`](./LICENSE) file.

**Note: This code is unaudited.**
In NO WAY should it be used to secure any monetary value before testing and auditing.
This is experimental software, and should be treated as such.
The authors of this project make no guarantees of security of ANY KIND.
