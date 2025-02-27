# Releases

## Versioning

For all releases after `v0.0.11`, each minor version of OP Deployer will support the current governance-approved
release of the smart contracts as well as the tip of the `develop` branch at the time the tag was created. If you
want to deploy an earlier version of the contracts (which may be dangerous!), you should use an earlier version of
OP Deployer. This setup allows our smart contract developers to make breaking changes on `develop`, while still
allowing new chains to be deployed and upgraded using production-ready smart contracts.

For example (note that these are just examples, check out the [releases][releases] page for the exact versions to use):

- `v0.1.x` : Supports deploying `develop` and `op-contracts/v2.0.0`.
- `v0.2.x`: Supports deploying `develop` and `op-contracts/v3.0.0`.

If you deploy from an HTTPS or file [locator](./artifacts-locators.md), the deployment behavior will match that of
the supported tag. For example, if you use `v0.1.x` then the deployment will work as if you were deploying
`op-contracts/v2.0.0`. Typically, errors like `unknown selector: <some hex>` imply that you're using the wrong
version of OP Deployer for your contract artifacts. If this happens, we recommend trying different versions until
you get one that works. Note that this workflow is **not recommended** for production chains.

[releases]: https://github.com/ethereum-optimism/optimism/releases

## Adding Support for New Contract Versions



Adding support for a new contract version is a multi-step process. Here's a high-level overview. For the sake of 
simplicity we will assume you are adding support for a new `rc` release.

### Step 1: Add Support on `develop`

**This section is designed for people developing OP Deployer itself.**

First, you need to add support for the new contract version on the `develop` branch. This means ensuring that the
deployment pipeline supports whatever changes are required for the new version. Typically, this means passing in new
deployment variables, and responding to ABI changes in the Solidity scripts/OPCM.

### Step 2: Add the Published Artifacts

Run the following from the root of the monorepo:

```bash
cd packages/contracts-bedrock
just clean
just build
bash scripts/ops/calculate-checksum.sh
# copy the outputted checksum
cd ../../op-deployer
just calculate-artifacts-hash <checksum>
```

This will calculate the checksum of your artifacts as well as the hash of the artifacts tarball. OP Deployer uses 
these values to download and verify tagged contract locators.

Now, update `standard/standard.go` with these values so that the new artifacts tarball can be downloaded:

```go
// Add a new const for your release

const ContractsVXTag = "op-contracts/vX.Y.Z"

var taggedReleases = map[string]TaggedRelease{
	// Other releases...
	ContractsVXTag: {
		ArtifactsHash: common.HexToHash("<the artifacts hash>"),
		ContentHash:   common.HexToHash("<the checksum>"),
	},
}

// Update the L1/L2 versions accordingly
func IsSupportedL1Version(tag string) bool {
	return tag == ContractsVXTag
}
```

### Step 3: Update the SR With the New Release

Add the new RC to the [standard versions][std-vers] in the Superchain Registry.

[std-vers]: https://github.com/ethereum-optimism/superchain-registry/tree/main/validation/standard

### Step 4: Update the `validation` Package

The SR is pulled into OP Deployer via the `validation` package. Update it by running the following command from the 
root of the monorepo:

```shell
go get -u github.com/ethereum-optimism/superchain-registry/validation@<SR commit SHA>
```

That should be it!