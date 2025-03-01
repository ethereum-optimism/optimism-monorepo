# The Bootstrap Commands

Bootstrap commands are used to deploy global singletons and implementation contracts for use with future invocations 
of `apply`. Most users won't need to use these commands, since `op-deployer apply` will automatically use 
predeployed contracts if they are available. However, you may need to use bootstrap commands if you're deploying 
chains to an L1 that isn't natively supported by `op-deployer`.

There are several bootstrap commands available, which you can view by running `op-deployer bootstrap --help`. We'll 
focus on the most important ones below.

## Implementations

You can bootstrap implementations by running a command like this:

```shell
op-deployer bootstrap implementations \
  --artifacts-locator <locator> \ 
  --l1-contracts-release op-contracts/<your-release> \
  --l1-rpc-url <rpc url> \
  --mips-version <1 or 2, for MIPS32 or MIPS64> \
  --private-key <some private key> \ 
  --protocol-versions-proxy <protocol versions proxy address> \
  --superchain-config-proxy <superchain config proxy address> \
  --upgrade-controller <upgrade controller address>
```

This command will deploy implementations, blueprints, and the OPCM. Deployments are (for the most part) 
deterministic, so contracts will only be deployed once per chain as long as the implementation and constructor args 
remain the same. This applies to the `op-deployer apply` pipeline - that is, if someone else ran `op-deployer 
boostrap implementations` at some point on a given L1 chain, then the `apply` pipeline will re-use those 
implementations.

The command will output a JSON like the one below:

```json
{
  "Opcm": "0x4eeb114aaf812e21285e5b076030110e7e18fed9",
  "DelayedWETHImpl": "0x5e40b9231b86984b5150507046e354dbfbed3d9e",
  "OptimismPortalImpl": "0x2d7e764a0d9919e16983a46595cfa81fc34fa7cd",
  "PreimageOracleSingleton": "0x1fb8cdfc6831fc866ed9c51af8817da5c287add3",
  "MipsSingleton": "0xf027f4a985560fb13324e943edf55ad6f1d15dc1",
  "SystemConfigImpl": "0x760c48c62a85045a6b69f07f4a9f22868659cbcc",
  "L1CrossDomainMessengerImpl": "0x3ea6084748ed1b2a9b5d4426181f1ad8c93f6231",
  "L1ERC721BridgeImpl": "0x276d3730f219f7ec22274f7263180b8452b46d47",
  "L1StandardBridgeImpl": "0x78972e88ab8bbb517a36caea23b931bab58ad3c6",
  "OptimismMintableERC20FactoryImpl": "0x5493f4677a186f64805fe7317d6993ba4863988f",
  "DisputeGameFactoryImpl": "0x4bba758f006ef09402ef31724203f316ab74e4a0",
  "AnchorStateRegistryImpl": "0x7b465370bb7a333f99edd19599eb7fb1c2d3f8d2",
  "SuperchainConfigImpl": "0x4da82a327773965b8d4d85fa3db8249b387458e7",
  "ProtocolVersionsImpl": "0x37e15e4d6dffa9e5e320ee1ec036922e563cb76c"
}
```

**It is safe to call this command from a hot wallet.** None of the contracts deployed by this command are "ownable," 
so the deployment address has no further control over the system.