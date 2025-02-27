# Standard Output Format

Kurtosis-devnet is tightly integrated with the [Optimism Devnet SDK](/devnet-sdk/). This integration is achieved through a standardized devnet descriptor format that enables powerful testing and automation capabilities.

## Accessing the Devnet Descriptor

The devnet descriptor is available in two ways:

1. **Deployment Output**
   - When you run any of the deployment commands (`just devnet ...`), the descriptor is printed to stdout
   - The output is a JSON file that fully describes your devnet configuration
   - You can capture this output for later use or automation

2. **Kurtosis Enclave Artifact**
   - The descriptor is also stored as a file artifact named "devnet" in the Kurtosis enclave
   - This allows other tools and services to discover and interact with your devnet
   - The descriptor can be accessed through devnet-sdk using the Kurtosis URL format: `kt://<enclave-name>/files/devnet`

Here's a simplified example of a devnet descriptor:

```json
{
  "l1": {
    "name": "Ethereum",
    "nodes": [
      {
        "services": {
          "cl": {
            "name": "cl-1-lighthouse-geth",
            "endpoints": {
              "http": {
                "host": "127.0.0.1",
                "port": 8545
              }
            }
          },
          "el": {
            "name": "el-1-geth-lighthouse",
            "endpoints": {
              "rpc": {
                "host": "127.0.0.1",
                "port": 8551
              }
            }
          }
        }
      }
    ],
    "addresses": {
      "l1CrossDomainMessenger": "0x...",
      "l1StandardBridge": "0x...",
      "optimismPortal": "0x..."
      // ... other contract addresses
    },
    "wallets": {
      "user-key-0": {
        "address": "0x...",
        "private_key": "0x..."
      }
      // ... other wallets
    },
    "jwt": "0x..."
  },
  "l2": [
    {
      "name": "op-kurtosis",
      "id": "2151908",
      "services": {
        "batcher": {
          "name": "op-batcher-op-kurtosis",
          "endpoints": {
            "http": {
              "host": "127.0.0.1",
              "port": 8547
            }
          }
        },
        "proposer": {
          "name": "op-proposer-op-kurtosis",
          "endpoints": {
            "http": {
              "host": "127.0.0.1",
              "port": 8548
            }
          }
        }
      },
      "nodes": [
        {
          "services": {
            "cl": {
              "name": "op-node",
              "endpoints": {
                "http": {
                  "host": "127.0.0.1",
                  "port": 8546
                }
              }
            },
            "el": {
              "name": "op-geth",
              "endpoints": {
                "rpc": {
                  "host": "127.0.0.1",
                  "port": 8549
                }
              }
            }
          }
        }
      ],
      "jwt": "0x..."
    }
  ]
}
```

This standardized output enables seamless integration with the devnet-sdk and other tools in the ecosystem.

## Devnet SDK Integration

By leveraging the devnet-sdk integration, your devnets automatically gain access to:

1. **Test Framework Integration**
   - Use your devnet as a System Under Test (SUT) with tests written in the devnet-sdk framework
   - Seamless integration with existing test suites
   - Standardized approach to devnet interaction in tests

2. **Test Runner Support**
   - Native support for op-nat as a test runner
   - Consistent test execution across different devnet configurations
   - Automated test setup and teardown

These capabilities make kurtosis-devnet an ideal platform for both development and testing environments.

## Devnet Descriptor Generation

In the implementation, the devnet descriptor file is of the type DevnetEnvironment, and is generated according to the following flow:

```mermaid
flowchart TD
    %% Main CLI entrypoint
    main_go["main.go (CLI entrypoint)"]
    cli_config["CLI Config Structure"]
    main_go --> |parses flags| cli_config

    %% Template and Data Files
    template_file["Template File (YAML)"]
    data_file["Optional JSON Data File"]
    cli_config --> |references| template_file
    cli_config --> |optional| data_file

    %% Deployer Creation
    deployer["Deployer"]
    cli_config --> |configures| deployer

    %% Template Processing
    templater["Templater"]
    deployer --> |creates| templater
    template_file --> |input to| templater
    data_file --> |optional input to| templater

    %% Template Functions
    templater --> |provides functions| template_functions["Template Functions"]
    template_functions --> |build docker images| local_docker["localDockerImage()"]
    template_functions --> |build contracts| local_contracts["localContractArtifacts()"]
    template_functions --> |generate prestate| local_prestate["localPrestate()"]

    %% Rendered Template
    rendered_buffer["Rendered Template Buffer"]
    templater --> |renders to| rendered_buffer

    %% Fileserver Deployment
    fileserver["FileServer"]
    deployer --> |deploys| fileserver

    %% Kurtosis Deployer
    kt_deployer["KurtosisDeployer"]
    deployer --> |creates| kt_deployer
    rendered_buffer --> |input to| kt_deployer

    %% Enclave Spec Parsing
    yaml_spec["YAML Spec"]
    enclave_spec["EnclaveSpec"]
    kt_deployer --> |parses| yaml_spec
    yaml_spec --> |converts to| enclave_spec

    %% Kurtosis Engine & Execution
    engine_manager["Engine Manager"]
    kt_runner["Kurtosis Runner"]
    deployer --> |starts| engine_manager
    kt_deployer --> |creates| kt_runner

    %% Environment Information Gathering
    inspector["Enclave Inspector"]
    observer["Enclave Observer"]
    jwt_extractor["JWT Extractor"]
    kt_deployer --> |uses| inspector
    kt_deployer --> |uses| observer
    kt_deployer --> |uses| jwt_extractor

    %% Service Discovery
    service_finder["Service Finder"]
    inspector --> |results used by| service_finder

    %% Service and Node Information
    l1_services["L1 Services/Nodes"]
    l2_services["L2 Services/Nodes"]
    service_finder --> |finds| l1_services
    service_finder --> |finds| l2_services

    %% Contract and Wallet Data
    addresses["Contract Addresses"]
    wallets["Wallet Data"]
    observer --> |extracts| addresses
    observer --> |extracts| wallets

    %% JWT Information
    jwt_data["JWT Data"]
    jwt_extractor --> |extracts| jwt_data

    %% Environment Construction
    kurtosis_env["KurtosisEnvironment"]
    l1_services --> |populates| kurtosis_env
    l2_services --> |populates| kurtosis_env
    addresses --> |populates| kurtosis_env
    wallets --> |populates| kurtosis_env
    jwt_data --> |populates| kurtosis_env
    enclave_spec -->|features| kurtosis_env

    %% Final Output
    env_output["Environment JSON"]
    kurtosis_env --> |serialized to| env_output

    %% Data Types
    classDef config fill:#f9f,stroke:#333,stroke-width:2px
    classDef process fill:#bbf,stroke:#333,stroke-width:2px
    classDef data fill:#ffb,stroke:#333,stroke-width:2px

    class cli_config,template_file,data_file,enclave_spec,yaml_spec config
    class main_go,deployer,templater,kt_deployer,kt_runner,engine_manager,fileserver process
    class rendered_buffer,l1_services,l2_services,addresses,wallets,jwt_data,kurtosis_env,env_output data
    class template_functions,local_docker,local_contracts,local_prestate,inspector,observer,jwt_extractor,service_finder process
```
