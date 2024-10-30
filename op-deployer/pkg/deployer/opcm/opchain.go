package opcm

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/broadcaster"

	"github.com/ethereum-optimism/optimism/op-chain-ops/foundry"
	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/holiman/uint256"
)

// PermissionedGameStartingAnchorRoots is a root of bytes32(hex"dead") for the permissioned game at block 0,
// and no root for the permissionless game.
var PermissionedGameStartingAnchorRoots = []byte{
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x20, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0xde, 0xad, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
}

type DeployOPChainInput struct {
	OpChainProxyAdminOwner common.Address
	SystemConfigOwner      common.Address
	Batcher                common.Address
	UnsafeBlockSigner      common.Address
	Proposer               common.Address
	Challenger             common.Address

	BasefeeScalar     uint32
	BlobBaseFeeScalar uint32
	L2ChainId         *big.Int
	OpcmProxy         common.Address
	SaltMixer         string
	GasLimit          uint64

	DisputeGameType              uint32
	DisputeAbsolutePrestate      common.Hash
	DisputeMaxGameDepth          uint64
	DisputeSplitDepth            uint64
	DisputeClockExtension        uint64
	DisputeMaxClockDuration      uint64
	AllowCustomDisputeParameters bool
}

func (input *DeployOPChainInput) InputSet() bool {
	return true
}

func (input *DeployOPChainInput) StartingAnchorRoots() []byte {
	return PermissionedGameStartingAnchorRoots
}

type DeployOPChainOutput struct {
	OpChainProxyAdmin                 common.Address
	AddressManager                    common.Address
	L1ERC721BridgeProxy               common.Address
	SystemConfigProxy                 common.Address
	OptimismMintableERC20FactoryProxy common.Address
	L1StandardBridgeProxy             common.Address
	L1CrossDomainMessengerProxy       common.Address
	// Fault proof contracts below.
	OptimismPortalProxy                common.Address
	DisputeGameFactoryProxy            common.Address
	AnchorStateRegistryProxy           common.Address
	AnchorStateRegistryImpl            common.Address
	FaultDisputeGame                   common.Address
	PermissionedDisputeGame            common.Address
	DelayedWETHPermissionedGameProxy   common.Address
	DelayedWETHPermissionlessGameProxy common.Address
}

func (output *DeployOPChainOutput) CheckOutput(input common.Address) error {
	return nil
}

type DeployOPChainScript struct {
	Run func(input, output common.Address) error
}

func DeployOPChain(host *script.Host, input DeployOPChainInput) (DeployOPChainOutput, error) {
	var dco DeployOPChainOutput
	inputAddr := host.NewScriptAddress()
	outputAddr := host.NewScriptAddress()

	cleanupInput, err := script.WithPrecompileAtAddress[*DeployOPChainInput](host, inputAddr, &input)
	if err != nil {
		return dco, fmt.Errorf("failed to insert DeployOPChainInput precompile: %w", err)
	}
	defer cleanupInput()
	host.Label(inputAddr, "DeployOPChainInput")

	cleanupOutput, err := script.WithPrecompileAtAddress[*DeployOPChainOutput](host, outputAddr, &dco,
		script.WithFieldSetter[*DeployOPChainOutput])
	if err != nil {
		return dco, fmt.Errorf("failed to insert DeployOPChainOutput precompile: %w", err)
	}
	defer cleanupOutput()
	host.Label(outputAddr, "DeployOPChainOutput")

	deployScript, cleanupDeploy, err := script.WithScript[DeployOPChainScript](host, "DeployOPChain.s.sol", "DeployOPChain")
	if err != nil {
		return dco, fmt.Errorf("failed to load DeployOPChain script: %w", err)
	}
	defer cleanupDeploy()

	if err := deployScript.Run(inputAddr, outputAddr); err != nil {
		return dco, fmt.Errorf("failed to run DeployOPChain script: %w", err)
	}

	return dco, nil
}

// opcmRoles is an internal struct used to pass the roles to OPSM. See opcmDeployInput for more info.
type opcmRoles struct {
	OpChainProxyAdminOwner common.Address
	SystemConfigOwner      common.Address
	Batcher                common.Address
	UnsafeBlockSigner      common.Address
	Proposer               common.Address
	Challenger             common.Address
}

// opcmDeployInput is the input struct for the deploy method of the OPStackManager contract. We
// define a separate struct here to match what the OPSM contract expects.
type opcmDeployInput struct {
	Roles                   opcmRoles
	BasefeeScalar           uint32
	BlobBasefeeScalar       uint32
	L2ChainId               *big.Int
	StartingAnchorRoots     []byte
	SaltMixer               string
	GasLimit                uint64
	DisputeGameType         uint32
	DisputeAbsolutePrestate common.Hash
	DisputeMaxGameDepth     *big.Int
	DisputeSplitDepth       *big.Int
	DisputeClockExtension   uint64
	DisputeMaxClockDuration uint64
}

// decodeOutputABIJSON defines an ABI for a fake method called "decodeOutput" that returns the
// DeployOutput struct. This allows the code in the deployer to decode directly into a struct
// using Geth's ABI library.
const decodeOutputABIJSON = `
[
  {
    "type": "function",
    "name": "decodeOutput",
    "inputs": [],
    "outputs": [
      {
        "name": "output",
        "indexed": false,
		"type": "tuple",
        "components": [
          {
            "name": "opChainProxyAdmin",
            "type": "address"
          },
          {
            "name": "addressManager",
            "type": "address"
          },
          {
            "name": "l1ERC721BridgeProxy",
            "type": "address"
          },
          {
            "name": "systemConfigProxy",
            "type": "address"
          },
          {
            "name": "optimismMintableERC20FactoryProxy",
            "type": "address"
          },
          {
            "name": "l1StandardBridgeProxy",
            "type": "address"
          },
          {
            "name": "l1CrossDomainMessengerProxy",
            "type": "address"
          },
          {
            "name": "optimismPortalProxy",
            "type": "address"
          },
          {
            "name": "disputeGameFactoryProxy",
            "type": "address"
          },
          {
            "name": "anchorStateRegistryProxy",
            "type": "address"
          },
          {
            "name": "anchorStateRegistryImpl",
            "type": "address"
          },
          {
            "name": "faultDisputeGame",
            "type": "address",
            "internalType": "contract FaultDisputeGame"
          },
          {
            "name": "permissionedDisputeGame",
            "type": "address"
          },
          {
            "name": "delayedWETHPermissionedGameProxy",
            "type": "address"
          },
          {
            "name": "delayedWETHPermissionlessGameProxy",
            "type": "address"
          }
        ]
      }
    ]
  }
]
`

var decodeOutputABI abi.ABI

// DeployOPChainRaw deploys an OP Chain using a raw call to a pre-deployed OPSM contract.
func DeployOPChainRaw(
	ctx context.Context,
	l1 *ethclient.Client,
	bcast broadcaster.Broadcaster,
	deployer common.Address,
	artifacts foundry.StatDirFs,
	input DeployOPChainInput,
) (DeployOPChainOutput, error) {
	var out DeployOPChainOutput

	artifactsFS := &foundry.ArtifactsFS{FS: artifacts}
	opcmArtifacts, err := artifactsFS.ReadArtifact("OPContractsManager.sol", "OPContractsManager")
	if err != nil {
		return out, fmt.Errorf("failed to read OPStackManager artifact: %w", err)
	}

	opcmABI := opcmArtifacts.ABI
	calldata, err := opcmABI.Pack("deploy", opcmDeployInput{
		Roles: opcmRoles{
			OpChainProxyAdminOwner: input.OpChainProxyAdminOwner,
			SystemConfigOwner:      input.SystemConfigOwner,
			Batcher:                input.Batcher,
			UnsafeBlockSigner:      input.UnsafeBlockSigner,
			Proposer:               input.Proposer,
			Challenger:             input.Challenger,
		},
		BasefeeScalar:           input.BasefeeScalar,
		BlobBasefeeScalar:       input.BlobBaseFeeScalar,
		L2ChainId:               input.L2ChainId,
		StartingAnchorRoots:     input.StartingAnchorRoots(),
		SaltMixer:               input.SaltMixer,
		GasLimit:                input.GasLimit,
		DisputeGameType:         input.DisputeGameType,
		DisputeAbsolutePrestate: input.DisputeAbsolutePrestate,
		DisputeMaxGameDepth:     new(big.Int).SetUint64(input.DisputeMaxGameDepth),
		DisputeSplitDepth:       new(big.Int).SetUint64(input.DisputeSplitDepth),
		DisputeClockExtension:   input.DisputeClockExtension,
		DisputeMaxClockDuration: input.DisputeMaxClockDuration,
	})
	if err != nil {
		return out, fmt.Errorf("failed to pack deploy input: %w", err)
	}

	nonce, err := l1.NonceAt(ctx, deployer, nil)
	if err != nil {
		return out, fmt.Errorf("failed to read nonce: %w", err)
	}

	bcast.Hook(script.Broadcast{
		From:  deployer,
		To:    input.OpcmProxy,
		Input: calldata,
		Value: (*hexutil.U256)(uint256.NewInt(0)),
		// use hardcoded 19MM gas for now since this is roughly what we've seen this deployment cost.
		GasUsed: 19_000_000,
		Type:    script.BroadcastCall,
		Nonce:   nonce,
	})

	results, err := bcast.Broadcast(ctx)
	if err != nil {
		return out, fmt.Errorf("failed to broadcast OP chain deployment: %w", err)
	}

	deployedEvent := opcmABI.Events["Deployed"]
	res := results[0]

	for _, log := range res.Receipt.Logs {
		if log.Topics[0] != deployedEvent.ID {
			continue
		}

		type EventData struct {
			DeployOutput []byte
		}
		var data EventData
		if err := opcmABI.UnpackIntoInterface(&data, "Deployed", log.Data); err != nil {
			return out, fmt.Errorf("failed to unpack Deployed event: %w", err)
		}

		type OutputData struct {
			Output DeployOPChainOutput
		}
		var outData OutputData
		if err := decodeOutputABI.UnpackIntoInterface(&outData, "decodeOutput", data.DeployOutput); err != nil {
			return out, fmt.Errorf("failed to unpack DeployOutput: %w", err)
		}

		return outData.Output, nil
	}

	return out, fmt.Errorf("failed to find Deployed event")
}

func init() {
	var err error
	decodeOutputABI, err = abi.JSON(strings.NewReader(decodeOutputABIJSON))
	if err != nil {
		panic(fmt.Sprintf("failed to parse decodeOutput ABI: %v", err))
	}
}
