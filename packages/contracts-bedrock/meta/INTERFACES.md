# INTERFACES.md

## Introduction

This document outlines the guidelines and best practices for using and creating interfaces in the
`contracts-bedrock` package.

## Importance of Interfaces

Interfaces are valuable for developers because:

1. They allow interaction with OP Stack contracts without importing the source code.
2. They provide compatibility across different compiler versions.
3. They can reduce contract compilation time.

### Example of Interface Usage

Instead of importing the full contract:

```solidity
import "./ComplexContract.sol";

contract MyContract {
    ComplexContract public complexContract;

    constructor(address _complexContractAddress) {
        complexContract = ComplexContract(_complexContractAddress);
    }

    function doSomething() external {
        complexContract.someFunction();
    }
}
```

You can use an interface:

```solidity
import "./interfaces/IComplexContract.sol";`

contract MyContract {
    IComplexContract public complexContract;

    constructor(address _complexContractAddress) {
        complexContract = IComplexContract(_complexContractAddress);
    }

    function doSomething() external {
        complexContract.someFunction();
    }
}
```

This approach allows for interaction without being tied to the specific implementation or compiler
version of `ComplexContract`.

## Current Interface Policy

### No Interface Imports in Source Contracts

Contrary to common practice, source contracts for which an interface is defined SHOULD NOT use the
interface contract. This means:

- `contract Whatever is IWhatever` is NOT allowed.
- Source contracts should not use types or other definitions from their interfaces.
- Contracts that build on base contracts (e.g., `contract OtherWhatever is Whatever`) should not
  import `IWhatever` or `IOtherWhatever`.

### Correct Implementation Example

Instead of:

```solidity
import "./IWhatever.sol";

contract Whatever is IWhatever {
    // Implementation
}
```

Do this:

```solidity
contract Whatever {
    // Direct implementation without importing interface
}
```

### Reasons for This Policy

1. **Automation Potential**: We aim to auto-generate interfaces in the future. Importing interfaces
  into source contracts would prevent this automation by creating a circular dependency.

2. **ABI Compatibility**: Achieving 1:1 compatibility between interface and source contract ABI
  becomes problematic when the source contract imports other contracts along with the interface.
  This is due to Solidity's handling of function redefinitions. See
  [Example of ABI Compatibility Issue](#example-of-abi-compatibility-issue) below for more context.

#### Example of ABI Compatibility Issue

```solidity
contract SomeBaseContract {
    event SomeEvent();
}

interface IWhatever {
    event SomeEvent();
    function someOtherFunction() external;
}

contract Whatever is IWhatever, SomeBaseContract {
    function someOtherFunction() external {}
}
```

In this case, Solidity will return the following compilation error:

```sh
DeclarationError: Event with same name and parameter types defined twice.
```

### Importing External Interfaces

Contracts CAN import interfaces for OTHER contracts. This practice helps mitigate compilation time
issues in older Solidity versions. As Solidity improves, we plan to phase out this exception.

#### Example of Allowed Interface Usage

```solidity
import "./IOtherContract.sol";

contract MyContract {
    IOtherContract public otherContract;

    constructor(address _otherContractAddress) {
        otherContract = IOtherContract(_otherContractAddress);
    }

    // Rest of the contract
}
```

## Creating Interfaces

You have several options for creating interfaces:

1. Use `cast interface`:

   ```sh
   cast interface ./path/to/contract/artifact.json
   ```

2. Use `forge inspect`:

   ```sh
   forge inspect <ContractName> abi --pretty
   ```

3. Create the interface manually:

   ```solidity
   interface IMyContract {
       function someFunction() external;
       function anotherFunction(uint256 _param) external returns (bool);
       // ... other functions and events
   }
   ```

Regardless of the method chosen, ensure that your ABIs are a 1:1 match with their source contracts.

NOTE: Using `cast interface` or `forge inspect` can fail to preserve certain types like `enum`
values. You may need to manually fix these issues or CI will complain.

## Verifying Interface Accuracy

To check if all interfaces match their source contracts:

- Run `just interface-check` or `just interface-check-no-build`

These commands will compare the ABIs of your interfaces with their corresponding source contracts and report any discrepancies.

## Future Goals

Our long-term objectives for interfaces include:

1. Automating interface generation
2. Using interfaces only for external users, not internally
3. Eliminating the need for interface imports in source contracts

Until we achieve these goals, we maintain the current policy to balance development efficiency and
compilation time improvements.
