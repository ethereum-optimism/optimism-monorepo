# Getting Started

Running a Kurtosis Devnet has the following prerequisites:
- Kurtosis must be installed
- Docker Desktop must be installed and running

Platform specific installation instructions for Kurtosis may be found [in Kurtosis documentation](https://docs.kurtosis.com/install/), alternatively Mac, Windows and Linux binaries can be found [here](https://github.com/kurtosis-tech/kurtosis-cli-release-artifacts).
For Mac users, the following command should suffice:
```
brew install kurtosis-tech/tap/kurtosis-cli
```
Check your Kurtosis version with `kurtosis version`. The current ideal version for these devnets is `1.4.3`.

Docker Desktop may be substituted by an alternative like Orbstack if you have that installed.

# Running A Devnet

To see available devnets, consult the `justfile` to see what `.*-devnet` targets exist, currently
- `simple-devnet`
- `interop-devnet`
- `user-devnet`

You can read over the referenced `yaml` files located in this directory to see the network definition which would be deployed. Mini and Simple are example network definitions, and User expects a provided network definition.

To run the Interop Devnet, simply:
```
just interop-devnet
```

If all works as expected, you should see a collection of containers appear in Docker. Some of them are Kurtosis infrastructure, while others are the actual hosts for your network. You can observe that the network is running by searching for "supervisor" and watching its logs.

## Resolving Issues

Here is a list of potential pitfalls when running Kurtosis and known solutions.

### `error ensuring kurtosis engine is running`
This error indicates Docker Desktop (or your alternative) is not running.

### `network with name kt-interop-devnet already exists`
If your kurtosis network is taken down and destroyed through docker, it is possible that the network resources are left around, preventing you from starting up a new network. To resolve, run:
```
kurtosis engine stop
docker network rm kt-interop-devnet
```

You can use `docker network ls` to inspect for networks to remove if the error message specifies some other network.

# Kurtosis-devnet support

## devnet specification

Due to sandboxing issues across repositories, we currently rely on a slight
superset of the native optimism-package specification YAML file, via go
templates.

So that means in particular that the regular optimism-package input is valid
here.

Additional custom functions:

- localDockerImage(PROJECT): builds a docker image for PROJECT based on the
  current branch content.

- localContractArtifacts(LAYER): builds a contracts bundle based on the current
  branch content (note: LAYER is currently ignored, we might need to revisit)

Example:

```yaml
...
  op_contract_deployer_params:
    image: {{ localDockerImage "op-deployer" }}
    l1_artifacts_locator: {{ localContractArtifacts "l1" }}
    l2_artifacts_locator: {{ localContractArtifacts "l2" }}
...
```

The list of supported PROJECT values can be found in `justfile` as a
PROJECT-image target. Adding a target there will immediately available to the
template engine.

## devnet deployment tool

Located in cmd/main.go, this tool handle the creation of an enclave matching the
provided specification.

The expected entry point for interacting with it is the corresponding
`just devnet SPEC` target.

This takes an optional 2nd argument, that can be used to provide values for the
template interpretation.

Note that a SPEC of the form `FOO.yaml` will yield a kurtosis enclave named
`FOO-devnet`

Convenience targets can be added to `justfile` for specific specifications, for
example:

```just
interop-devnet: (devnet "interop.yaml")
```

## devnet output

One important aspect of the devnet workflow is that the output should be
*consumable*. Going forward we want to integrate them into larger worfklows
(serving as targets for tests for example, or any other form of automation).

To address this, the deployment tool outputs a document with (hopefully!) useful
information. Here's a short extract:

```json
{
  "l1": {
    "name": "Ethereum",
    "nodes": [
      {
        "cl": "http://localhost:53689",
        "el": "http://localhost:53620"
      }
    ]
  },
  "l2": [
    {
      "name": "op-kurtosis-1",
      "id": "2151908",
      "services": {
        "batcher": "http://localhost:57259"
      },
      "nodes": [
        {
          "cl": "http://localhost:57029",
          "el": "http://localhost:56781"
        }
      ],
      "addresses": {
        "addressManager": "0x1b89c03f2d8041b2ba16b5128e613d9279195d1a",
        ...
      }
    },
    ...
  ],
  "wallets": {
    "baseFeeVaultRecipient": {
      "address": "0xF435e3ba80545679CfC24E5766d7B02F0CCB5938",
      "private_key": "0xc661dd5d4b091676d1a5f2b5110f9a13cb8682140587bd756e357286a98d2c26"
    },
    ...
  }
}
```

## further interactions

Beyond deployment, we can interact with enclaves normally.

In particular, cleaning up a devnet can be achieved using
`kurtosis rm FOO-devnet` and the likes.
