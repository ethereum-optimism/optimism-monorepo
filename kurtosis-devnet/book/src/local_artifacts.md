# Local Artifacts Integration

The Kurtosis devnet provides powerful templating capabilities that allow you to seamlessly integrate locally built artifacts (Docker images, smart contracts, and prestates) into your devnet configuration. This integration is managed through a combination of Go-based builders and YAML templates.

## Component Eligibility

Not all components can be built locally. Only components that are part of the Optimism monorepo can be built using the local artifact system. Here's a breakdown:

### Buildable Components
Components that can be built locally include:
- `op-node`
- `op-batcher`
- `op-proposer`
- `op-challenger`
- `op-deployer`

### External Components
Some components are dependencies living outside the monorepo and cannot be built locally:
- `op-geth`
- `op-reth`

For example, in your configuration:
```yaml
# This will use an external image - cannot be built locally
el_type: op-geth
el_image: ""  # Will use the default op-geth image

# This can be built locally
cl_type: op-node
cl_image: {{ localDockerImage "op-node" }}  # Will build from local source
```

## Template Functions

In the `simple.yaml` configuration, you'll notice several custom template functions that enable local artifact integration:

```yaml
# Example usage in simple.yaml
image: {{ localDockerImage "op-node" }}
l1_artifacts_locator: {{ localContractArtifacts "l1" }}
faultGameAbsolutePrestate: {{ localPrestate.Hashes.prestate }}
```

These template functions map to specific builders in the Go codebase that handle artifact construction.

## Builder Components

### 1. Docker Image Builder

The Docker image builder manages the building and tagging of local Docker images:

```go
// Usage in YAML:
image: {{ localDockerImage "op-node" }}
```

This builder:
- Executes build commands using the `just` task runner
- Caches built images to prevent redundant builds (in particular when we have multiple L2s and/or participants to any L2)

### 2. Contract Builder

The contract builder handles the compilation and bundling of smart contracts:

```yaml
# Usage in YAML:
l1_artifacts_locator: {{ localContractArtifacts "l1" }}
l2_artifacts_locator: {{ localContractArtifacts "l2" }}
```

This builder:
- Manages contract compilation through `just` commands
- Caches built contract bundles

### 3. Prestate Builder

The prestate builder manages the generation of fault proof prestates:

```yaml
# Usage in YAML:
faultGameAbsolutePrestate: {{ localPrestate.Hashes.prestate }}
```

This builder:
- Generates prestate data for fault proofs
- Caches built prestates

## Using Local Artifacts

To use local artifacts in your devnet:

1. Ensure your local environment has the necessary build dependencies
2. Reference local artifacts in your YAML configuration using the appropriate template functions
3. The builders will automatically handle building and caching of artifacts

Example configuration using all types of local artifacts:

```yaml
optimism_package:
  chains:
    - participants:
        - el_type: op-geth
          el_image: ""  # Uses default external op-geth image
          cl_type: op-node
          cl_image: {{ localDockerImage "op-node" }}
  op_contract_deployer_params:
    image: {{ localDockerImage "op-deployer" }}
    l1_artifacts_locator: {{ localContractArtifacts "l1" }}
    l2_artifacts_locator: {{ localContractArtifacts "l2" }}
    global_deploy_overrides:
      faultGameAbsolutePrestate: {{ localPrestate.Hashes.prestate }}
```

This integration system ensures that your devnet can seamlessly use locally built components while maintaining reproducibility and ease of configuration.
