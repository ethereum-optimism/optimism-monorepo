# The Apply Command

Once you have [initialized][init] your intent and state files, you can use the `apply` command to perform the
deployment.

[init]: init.md

You can call the `apply` command like this:

```shell

op-deployer apply \
  --workdir <directory containing the intent and state files> \
  <... additional arguments ...>
```

You will need to specify additional arguments depending on what you're trying to do. See below for a reference of each
supported CLI arg.

### `--deployment-target`

**Default:** `live`

`--deployment-target` specifies where each chain should be deployed to. It can be one of the following values:

- `live`: Deploys to a live L1. Concretely, this means that OP Deployer will send transactions identified by
  `vm.broadcast` calls to L1. `--l1-rpc-url` and `--private-key` must be specified when using this target.
- `genesis`: Deploys to an L1 genesis file. This is useful for testing or local development purposes. You do not need to
  specify any additional arguments when using this target.
- `calldata`: Deploys to a calldata file. This is useful for generating inputs to multisig wallets for future execution.
- `noop`: Doesn't deploy anything. This is useful for performing a dry-run of the deployment process prior to another
  deployment target.

### `--l1-rpc-url`

Defines the RPC URL of the L1 chain to deploy to.

### `--private-key`

Defines the private key to use for signing transactions. This is only required for deployment targets that involve
sending live transactions. Note that ownership over each L2 is transferred to the proxy admin owner specified in the
intent after the deployment completes, so it's OK to use a hot key for this purpose.