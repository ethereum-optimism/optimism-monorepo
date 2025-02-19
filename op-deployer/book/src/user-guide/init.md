# The Init Command

The `init` command is used to create a new intent and state file in the specified directory. This command is the
starting point of each new deployment.

The `init` command is used like this:

```shell
op-deployer init \
  --l1-chain-id <chain ID of your L1> \
  --l2-chain-ids <comman separated list of chain IDs for your L2s> \
  --outdir <directory to write the intent and state files> \
  --intent-type <standard/custom/standard-overrides>
```

You should then see the following files appear in your output directory:

```
outdir
├── intent.toml
└── state.json
```

The `intent.toml` file is where you specify the configuration for your deployment. The `state.json` file is where OP
Deployer will output the current state of the deployment after each [stage][stages] of the deployment.

Your intent should look something like this:

```toml
configType = "standard"
l1ChainID = 11155420
fundDevAccounts = false
useInterop = false
l1ContractsLocator = "tag://op-contracts/v1.8.0-rc.4"
l2ContractsLocator = "op-contracts/v1.7.0-beta.1+l2-contracts"

[superchainRoles]
  proxyAdminOwner = "0xeAAA3fd0358F476c86C26AE77B7b89a069730570"
  protocolVersionsOwner = "0xeAAA3fd0358F476c86C26AE77B7b89a069730570"
  guardian = "0xeAAA3fd0358F476c86C26AE77B7b89a069730570"

[[chains]]
  id = "0x0000000000000000000000000000000000000000000000000000000000002390"
  baseFeeVaultRecipient = "0x0000000000000000000000000000000000000000"
  l1FeeVaultRecipient = "0x0000000000000000000000000000000000000000"
  sequencerFeeVaultRecipient = "0x0000000000000000000000000000000000000000"
  eip1559DenominatorCanyon = 250
  eip1559Denominator = 50
  eip1559Elasticity = 6
  [chains.roles]
    l1ProxyAdminOwner = "0x0000000000000000000000000000000000000000"
    l2ProxyAdminOwner = "0x0000000000000000000000000000000000000000"
    systemConfigOwner = "0x0000000000000000000000000000000000000000"
    unsafeBlockSigner = "0x0000000000000000000000000000000000000000"
    batcher = "0x0000000000000000000000000000000000000000"
    proposer = "0x0000000000000000000000000000000000000000"
    challenger = "0x0000000000000000000000000000000000000000"
```

Before you can use your intent file for a deployment, you will need to update all zero values to whatever is
appropriate for your chain.

[stages]: ../architecture/pipeline.md
