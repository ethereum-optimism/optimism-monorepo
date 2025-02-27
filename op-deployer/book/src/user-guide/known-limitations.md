# Known Limitations

OP Deployer is subject to some known limitations which we're working on addressing in future releases.

## Tagged Releases on New Chains

It is not currently possible to deploy chains using tagged contract locators (i.e., those starting with `tag://`)
anywhere except Sepolia and Ethereum mainnet. If you try to, you'll see an error like this:

```
####################### WARNING! WARNING WARNING! #######################

You are deploying a tagged release to a chain with no pre-deployed OPCM.
Due to a quirk of our contract version system, this can lead to deploying
contracts containing unaudited or untested code. As a result, this 
functionality is currently disabled.

We will fix this in an upcoming release.

This process will now exit.

####################### WARNING! WARNING WARNING! #######################
```

Like the error says, this is due to a quirk of how we version our smart contracts. We currently follow a process
like this:

1. We tag a release, like op-contracts/v1.8.0.
2. We update the [release notes][release-notes] to reference which contracts are updated in that release.
3. We manually deploy the updated contract implementations.
4. We manually deploy a new OPCM to reference the newly-deployed implementations, as well as existing implementations
   for any contracts that have not been updated.

There's a flaw in this strategy, however. The release only includes the contracts that explicitly changed during
that release. **This means that any contract not referenced as "updated" in the release notes is "in-development," and
has not been audited or approved by governance.** Deploying all contracts from the release tag will therefore deploy a
combination of prod-ready and in-development code. To get the version of the contract that will _actually_ run in prod,
OP Deployer would have to reference all previous releases to get the correct combination of contracts.

For example, to deploy on Holesky you will need to deploy contracts from versions `op-contracts/v1.8.0`, `op-contracts/v1.6.0`, and `op-contracts/v1.3.0`. On
Sepolia and mainnet, we've been incrementally deploying implementation contracts so we just use the existing
¬implementations to work around this issue.

We plan on addressing this in our next release. In the meantime, as a workaround you can use a non-tagged locator for
development chains, or use Sepolia or Ethereum mainnet as your L1.

[release-notes]: https://github.com/ethereum-optimism/optimism/releases/tag/op-contracts%2Fv1.8.0