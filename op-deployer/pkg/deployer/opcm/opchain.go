package opcm

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	_ "embed"

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

// opcmRolesBase is an internal struct used to pass the roles to OPCM. See opcmDeployInputV160 for more info.
type opcmRolesBase struct {
	OpChainProxyAdminOwner common.Address
	SystemConfigOwner      common.Address
	Batcher                common.Address
	UnsafeBlockSigner      common.Address
	Proposer               common.Address
	Challenger             common.Address
}

type opcmDeployInputBase struct {
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

// opcmDeployInputV160 is the input struct for the deploy method of the OPStackManager contract. We
// define a separate struct here to match what the OPSM contract expects.
type opcmDeployInputV160 struct {
	opcmDeployInputBase
	Roles opcmRolesBase
}

type opcmRolesIsthmus struct {
	opcmRolesBase
	FeeAdmin common.Address
}

type opcmDeployInputIsthmus struct {
	opcmDeployInputBase
	Roles opcmRolesIsthmus
}

type DeployOPChainInputV160 struct {
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

func (input *DeployOPChainInputV160) InputSet() bool {
	return true
}

func (input *DeployOPChainInputV160) StartingAnchorRoots() []byte {
	return PermissionedGameStartingAnchorRoots
}

func DeployOPChainInputV160DeployCalldata(input DeployOPChainInputV160) any {
	return opcmDeployInputV160{
		Roles: opcmRolesBase{
			OpChainProxyAdminOwner: input.OpChainProxyAdminOwner,
			SystemConfigOwner:      input.SystemConfigOwner,
			Batcher:                input.Batcher,
			UnsafeBlockSigner:      input.UnsafeBlockSigner,
			Proposer:               input.Proposer,
			Challenger:             input.Challenger,
		},
		opcmDeployInputBase: opcmDeployInputBase{
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
		},
	}
}

type DeployOPChainInputIsthmus struct {
	DeployOPChainInputV160
	SystemConfigFeeAdmin common.Address
}

func DeployOPChainInputIsthmusDeployCalldata(input DeployOPChainInputIsthmus) any {
	v160Data := DeployOPChainInputV160DeployCalldata(input.DeployOPChainInputV160).(opcmDeployInputV160)
	return opcmDeployInputIsthmus{
		Roles: opcmRolesIsthmus{
			opcmRolesBase: v160Data.Roles,
			FeeAdmin:      input.SystemConfigFeeAdmin,
		},
		opcmDeployInputBase: v160Data.opcmDeployInputBase,
	}
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

func DeployOPChainV160(host *script.Host, input DeployOPChainInputV160) (DeployOPChainOutput, error) {
	return deployOPChain(host, input)
}

func DeployOPChainIsthmus(host *script.Host, input DeployOPChainInputIsthmus) (DeployOPChainOutput, error) {
	return deployOPChain(host, input)
}

func deployOPChain[T any](host *script.Host, input T) (DeployOPChainOutput, error) {
	var dco DeployOPChainOutput
	inputAddr := host.NewScriptAddress()
	outputAddr := host.NewScriptAddress()

	cleanupInput, err := script.WithPrecompileAtAddress[*T](host, inputAddr, &input)
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

// decodeOutputABIJSONV160 defines an ABI for a fake method called "decodeOutput" that returns the
// DeployOutput struct. This allows the code in the deployer to decode directly into a struct
// using Geth's ABI library.
//
//go:embed deployOutput_v160.json
var decodeOutputABIJSONV160 string

var decodeOutputABIV160 abi.ABI

func DeployOPChainRawV160(
	ctx context.Context,
	l1 *ethclient.Client,
	bcast broadcaster.Broadcaster,
	deployer common.Address,
	artifacts foundry.StatDirFs,
	input DeployOPChainInputV160,
) (DeployOPChainOutput, error) {
	return deployOPChainRaw(ctx, l1, bcast, deployer, artifacts, input.OpcmProxy, DeployOPChainInputV160DeployCalldata(input))
}

func DeployOPChainRawIsthmus(
	ctx context.Context,
	l1 *ethclient.Client,
	bcast broadcaster.Broadcaster,
	deployer common.Address,
	artifacts foundry.StatDirFs,
	input DeployOPChainInputIsthmus,
) (DeployOPChainOutput, error) {
	return deployOPChainRaw(ctx, l1, bcast, deployer, artifacts, input.OpcmProxy, DeployOPChainInputIsthmusDeployCalldata(input))
}

// DeployOPChainRaw deploys an OP Chain using a raw call to a pre-deployed OPSM contract.
func deployOPChainRaw(
	ctx context.Context,
	l1 *ethclient.Client,
	bcast broadcaster.Broadcaster,
	deployer common.Address,
	artifacts foundry.StatDirFs,
	opcmProxyAddress common.Address,
	input any,
) (DeployOPChainOutput, error) {
	var out DeployOPChainOutput

	artifactsFS := &foundry.ArtifactsFS{FS: artifacts}
	opcmArtifacts, err := artifactsFS.ReadArtifact("OPContractsManager.sol", "OPContractsManager")
	if err != nil {
		return out, fmt.Errorf("failed to read OPStackManager artifact: %w", err)
	}

	opcmABI := opcmArtifacts.ABI
	calldata, err := opcmABI.Pack("deploy", input)
	if err != nil {
		return out, fmt.Errorf("failed to pack deploy input: %w", err)
	}

	nonce, err := l1.NonceAt(ctx, deployer, nil)
	if err != nil {
		return out, fmt.Errorf("failed to read nonce: %w", err)
	}

	bcast.Hook(script.Broadcast{
		From:  deployer,
		To:    opcmProxyAddress,
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
		if err := decodeOutputABIV160.UnpackIntoInterface(&outData, "decodeOutput", data.DeployOutput); err != nil {
			return out, fmt.Errorf("failed to unpack DeployOutput: %w", err)
		}

		return outData.Output, nil
	}

	return out, fmt.Errorf("failed to find Deployed event")
}

func init() {
	var err error
	decodeOutputABIV160, err = abi.JSON(strings.NewReader(decodeOutputABIJSONV160))
	if err != nil {
		panic(fmt.Sprintf("failed to parse decodeOutput ABI: %v", err))
	}
}
