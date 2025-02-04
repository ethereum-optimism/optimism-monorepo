# Scripting Engine

One of OP Deployer's most powerful features is its in-memory EVM scripting engine. The scripting engine provides
similar capabilities to Forge:

- It runs all on-chain calls in a simulated environment first, which allows the effects of on-chain calls to be
  validated before they cost gas.
- It exposes Foundry cheatcodes, which allow for deep instrumentation and customization of the EVM environment. These
  cheatcodes in turn allow OP Deployer to call into Solidity scripts.

The scripting engine is really the heart of OP Deployer. Without it, OP Deployer would be nothing more than a thin
wrapper over Forge. The scripting engine enables:

- Easy integration with existing Solidity-based tooling.
- Detailed stack traces when deployments fail.
- Fast feedback loops that prevent sending on-chain transactions that may fail.
- Live chain forking.

For these reasons and more, the scripting engine is a critical part of OP Deployer's architecture. You will see that
almost all on-chain interactions initiated by OP Deployer use the scripting engine to call into a Solidity script.
The script then uses `vm.broadcast` to signal a transaction that should be sent on-chain.

### Aside: Why Use Solidity Scripts?

Solidity scripts are much more ergonomic than Go code for complex on-chain interactions. They allow for:

- Easy integration with existing Solidity-based tooling and libraries.
- Simple ABI encoding/decoding.
- Clear separation of concerns between inter-contract calls, and the underlying RPC calls that drive them.

The alternative is to encode all on-chain interactions in Go code. This is possible, but it is much more verbose and 
requires writing bindings between Go and the Solidity ABI. These bindings are error-prone and difficult to maintain.

## Engine Implementation

The scripting engine is implemented in the `op-chain-ops/script` package. It extends Geth's EVM implementation with
Forge cheatcodes, and defines some tools that allow Go structs to be etched into the EVM's memory. Geth exposes
hooks that drive most of the engine's behavior. The best way to understand these further is to read the code.

## Using the Engine

OP Deployer uses the etching tooling described above to communicate between OP Deployer and the scripting engine. 
Most Solidity scripts define an input contract, an output contract, and the script itself. The script reads data 
from fields on the input contract, then sets fields on the output contract as it runs. OP Deployer defines the input 
and output contracts as Go structs, like this:

```go
package foo_script

type FooInput struct {
  Number uint64
  Bytes []byte
}

type FooOutput struct {
  Result uint64
  Bytes []byte
}
```

The input and output contracts are then "etched" into the EVM's memory, like this:

```go
package foo_script

// ... struct defs elided

func Run(host *script.Host, input FooInput) (FooOutput, error) {
	// Create a variable to hold our output
	var output FooOutput
	
	// Make new addresses for our input/output contracts
	inputAddr := host.NewScriptAddress()
	outputAddr := host.NewScriptAddress()

	// Inject the input/output contracts into the EVM as precompiles
	cleanupInput, err := script.WithPrecompileAtAddress[*FooInput](host, inputAddr, &input)
	if err != nil {
		return output, fmt.Errorf("failed to insert input precompile: %w", err)
	}
	defer cleanupInput()

	cleanupOutput, err := script.WithPrecompileAtAddress[*FooOutput](host, outputAddr, &output,
		script.WithFieldSetter[*FooOutput])
	if err != nil {
		return output, fmt.Errorf("failed to insert output precompile: %w", err)
	}
	defer cleanupOutput()
	
	// ... do stuff with the input/output contracts ...
}
```

The script engine will automatically generate getters and setters for the fields on the input and output contracts. 
You can use the `evm:` struct tag to customize the behavior of these getters and setters.

Finally, the script itself gets etched into the EVM's memory and executed, like this:

```go
package foo_script

type FooScript struct {
	Run func(input, output common.Address) error
}

func Run(host *script.Host, input FooInput) (FooOutput, error) {
	// .. see implementation above...

	deployScript, cleanupDeploy, err := script.WithScript[FooScript](host, "FooScript.s.sol", "FooScript")
	if err != nil {
		return output, fmt.Errorf("failed to load %s script: %w", scriptFile, err)
	}
	defer cleanupDeploy()

	if err := deployScript.Run(inputAddr, outputAddr); err != nil {
		return output, fmt.Errorf("failed to run %s script: %w", scriptFile, err)
	}

	return output, nil
}
```

You may notice that the script is loaded from a file. To run the scripting engine, contract artifacts (**not** 
source code) must exist somewhere on disk for the scripting engine to use. For more information on that, see the 
chapter on artifacts locators.