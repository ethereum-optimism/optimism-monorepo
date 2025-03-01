package verify

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/lmittmann/w3"
	"github.com/lmittmann/w3/module/eth"
)

type constructorArgEncoder func(*Verifier) (string, error)

var constructorArgEncoders = map[string]constructorArgEncoder{
	"ProxyAdminAddress":              encodeProxyAdminArgs,
	"OpcmAddress":                    encodeOpcmArgs,
	"DelayedWETHImplAddress":         encodeDelayedWETHArgs,
	"OptimismPortalImplAddress":      encodeOptimismPortalArgs,
	"PreimageOracleSingletonAddress": encodePreimageOracleArgs,
	"MipsSingletonAddress":           encodeMipsArgs,
	"SuperchainConfigProxyAddress":   encodeSuperchainConfigProxyArgs,
	"PermissionedDisputeGameAddress": encodePermissionedDisputeGameArgs,
}

func (v *Verifier) getEncodedConstructorArgs(contractName string) (string, error) {
	encoder, exists := constructorArgEncoders[contractName]
	if !exists {
		return "", nil
	}
	return encoder(v)
}

func encodeProxyAdminArgs(v *Verifier) (string, error) {
	addr := v.st.AppliedIntent.SuperchainRoles.ProxyAdminOwner
	padded := common.LeftPadBytes(addr.Bytes(), 32)
	return hexutil.Encode(padded)[2:], nil
}

func encodeDelayedWETHArgs(v *Verifier) (string, error) {
	var withdrawalDelay big.Int
	withdrawalDelayFn := w3.MustNewFunc("delay()", "uint256")
	if err := v.w3Client.Call(
		eth.CallFunc(v.st.ImplementationsDeployment.DelayedWETHImplAddress, withdrawalDelayFn).Returns(&withdrawalDelay),
	); err != nil {
		return "", err
	}
	paddedDelay := common.LeftPadBytes(withdrawalDelay.Bytes(), 32)
	return strings.TrimPrefix(hexutil.Encode(paddedDelay), "0x"), nil
}

func encodeOptimismPortalArgs(v *Verifier) (string, error) {
	var maturityDelay big.Int
	proofMaturityDelayFn := w3.MustNewFunc("proofMaturityDelaySeconds()", "uint256")
	if err := v.w3Client.Call(
		eth.CallFunc(v.st.ImplementationsDeployment.OptimismPortalImplAddress, proofMaturityDelayFn).Returns(&maturityDelay),
	); err != nil {
		return "", err
	}

	var finalityDelay big.Int
	disputeGameFinalityDelayFn := w3.MustNewFunc("disputeGameFinalityDelaySeconds()", "uint256")
	if err := v.w3Client.Call(
		eth.CallFunc(v.st.ImplementationsDeployment.OptimismPortalImplAddress, disputeGameFinalityDelayFn).Returns(&finalityDelay),
	); err != nil {
		return "", err
	}

	paddedMaturity := common.LeftPadBytes(maturityDelay.Bytes(), 32)
	paddedFinality := common.LeftPadBytes(finalityDelay.Bytes(), 32)
	concatenated := append(paddedMaturity, paddedFinality...)
	return strings.TrimPrefix(hexutil.Encode(concatenated), "0x"), nil
}

func encodePreimageOracleArgs(v *Verifier) (string, error) {
	var minProposalSize big.Int
	minProposalSizeFn := w3.MustNewFunc("minProposalSize()", "uint256")
	if err := v.w3Client.Call(
		eth.CallFunc(v.st.ImplementationsDeployment.PreimageOracleSingletonAddress, minProposalSizeFn).Returns(&minProposalSize),
	); err != nil {
		return "", err
	}

	var challengePeriod big.Int
	challengePeriodFn := w3.MustNewFunc("challengePeriod()", "uint256")
	if err := v.w3Client.Call(
		eth.CallFunc(v.st.ImplementationsDeployment.PreimageOracleSingletonAddress, challengePeriodFn).Returns(&challengePeriod),
	); err != nil {
		return "", err
	}

	paddedMinProposalSize := common.LeftPadBytes(minProposalSize.Bytes(), 32)
	paddedChallengePeriod := common.LeftPadBytes(challengePeriod.Bytes(), 32)
	concatenated := append(paddedMinProposalSize, paddedChallengePeriod...)
	return strings.TrimPrefix(hexutil.Encode(concatenated), "0x"), nil
}

func encodeMipsArgs(v *Verifier) (string, error) {
	addr := v.st.ImplementationsDeployment.PreimageOracleSingletonAddress
	padded := common.LeftPadBytes(addr.Bytes(), 32)
	return hexutil.Encode(padded)[2:], nil
}

func encodeOpcmArgs(v *Verifier) (string, error) {
	type Blueprints struct {
		AddressManager             common.Address `abi:"field0"`
		Proxy                      common.Address `abi:"field1"`
		ProxyAdmin                 common.Address `abi:"field2"`
		L1ChugSplashProxy          common.Address `abi:"field3"`
		ResolvedDelegateProxy      common.Address `abi:"field4"`
		PermissionedDisputeGame1   common.Address `abi:"field5"`
		PermissionedDisputeGame2   common.Address `abi:"field6"`
		PermissionlessDisputeGame1 common.Address `abi:"field7"`
		PermissionlessDisputeGame2 common.Address `abi:"field8"`
	}

	type Implementations struct {
		SuperchainConfigImpl             common.Address `abi:"field0"`
		ProtocolVersionsImpl             common.Address `abi:"field1"`
		L1ERC721BridgeImpl               common.Address `abi:"field2"`
		OptimismPortalImpl               common.Address `abi:"field3"`
		SystemConfigImpl                 common.Address `abi:"field4"`
		OptimismMintableERC20FactoryImpl common.Address `abi:"field5"`
		L1CrossDomainMessengerImpl       common.Address `abi:"field6"`
		L1StandardBridgeImpl             common.Address `abi:"field7"`
		DisputeGameFactoryImpl           common.Address `abi:"field8"`
		AnchorStateRegistryImpl          common.Address `abi:"field9"`
		DelayedWETHImpl                  common.Address `abi:"field10"`
		MipsImpl                         common.Address `abi:"field11"`
	}

	var blueprints Blueprints
	blueprintsFn := w3.MustNewFunc("blueprints()", "(address addressManager,address proxy,address proxyAdmin,address l1ChugSplashProxy,address resolvedDelegateProxy,address permissionedDisputeGame1,address permissionedDisputeGame2,address permissionlessDisputeGame1,address permissionlessDisputeGame2)")
	if err := v.w3Client.Call(eth.CallFunc(v.st.ImplementationsDeployment.OpcmAddress, blueprintsFn).Returns(&blueprints)); err != nil {
		return "", err
	}

	var impls Implementations
	implementationsFn := w3.MustNewFunc("implementations()", "(address superchainConfigImpl,address protocolVersionsImpl,address l1ERC721BridgeImpl,address optimismPortalImpl,address systemConfigImpl,address optimismMintableERC20FactoryImpl,address l1CrossDomainMessengerImpl,address l1StandardBridgeImpl,address disputeGameFactoryImpl,address anchorStateRegistryImpl,address delayedWETHImpl,address mipsImpl)")
	if err := v.w3Client.Call(eth.CallFunc(v.st.ImplementationsDeployment.OpcmAddress, implementationsFn).Returns(&impls)); err != nil {
		return "", err
	}

	var release string
	releaseFn := w3.MustNewFunc("l1ContractsRelease()", "string")
	if err := v.w3Client.Call(eth.CallFunc(v.st.ImplementationsDeployment.OpcmAddress, releaseFn).Returns(&release)); err != nil {
		return "", err
	}

	var isRc bool
	isRcFn := w3.MustNewFunc("isRC()", "bool")
	if err := v.w3Client.Call(eth.CallFunc(v.st.ImplementationsDeployment.OpcmAddress, isRcFn).Returns(&isRc)); err != nil {
		return "", err
	}
	if isRc {
		// Opcm code appends the "-rc" suffix, so we need to remove it to recreate the constructor arg
		release = strings.TrimSuffix(release, "-rc")
	}

	var upgradeController common.Address
	upgradeControllerFn := w3.MustNewFunc("upgradeController()", "address")
	if err := v.w3Client.Call(eth.CallFunc(v.st.ImplementationsDeployment.OpcmAddress, upgradeControllerFn).Returns(&upgradeController)); err != nil {
		return "", err
	}

	var superchainConfig common.Address
	superchainConfigFn := w3.MustNewFunc("superchainConfig()", "address")
	if err := v.w3Client.Call(eth.CallFunc(v.st.ImplementationsDeployment.OpcmAddress, superchainConfigFn).Returns(&superchainConfig)); err != nil {
		return "", err
	}

	var protocolVersions common.Address
	protocolVersionsFn := w3.MustNewFunc("protocolVersions()", "address")
	if err := v.w3Client.Call(eth.CallFunc(v.st.ImplementationsDeployment.OpcmAddress, protocolVersionsFn).Returns(&protocolVersions)); err != nil {
		return "", err
	}

	var superchainProxyAdmin common.Address
	superchainProxyAdminFn := w3.MustNewFunc("superchainProxyAdmin()", "address")
	if err := v.w3Client.Call(eth.CallFunc(v.st.ImplementationsDeployment.OpcmAddress, superchainProxyAdminFn).Returns(&superchainProxyAdmin)); err != nil {
		return "", err
	}

	result := []byte{}
	result = append(result, common.LeftPadBytes(superchainConfig.Bytes(), 32)...)
	result = append(result, common.LeftPadBytes(protocolVersions.Bytes(), 32)...)
	result = append(result, common.LeftPadBytes(superchainProxyAdmin.Bytes(), 32)...)

	// Calculate dynamic offset for _l1ContractsRelease.
	// 3 addresses
	// 1 dynamic offset for _l1ContractsRelease,
	// 9 addresses for blueprints
	// 12 addresses for implementations
	// 1 address for _upgradeController.
	// --------------------------------
	// Total: 26 slots
	// Offset = 26 * 32 = 832 bytes.
	offset := big.NewInt(26 * 32) // 832
	result = append(result, common.LeftPadBytes(offset.Bytes(), 32)...)

	// blueprints
	result = append(result, common.LeftPadBytes(blueprints.AddressManager.Bytes(), 32)...)
	result = append(result, common.LeftPadBytes(blueprints.Proxy.Bytes(), 32)...)
	result = append(result, common.LeftPadBytes(blueprints.ProxyAdmin.Bytes(), 32)...)
	result = append(result, common.LeftPadBytes(blueprints.L1ChugSplashProxy.Bytes(), 32)...)
	result = append(result, common.LeftPadBytes(blueprints.ResolvedDelegateProxy.Bytes(), 32)...)
	result = append(result, common.LeftPadBytes(blueprints.PermissionedDisputeGame1.Bytes(), 32)...)
	result = append(result, common.LeftPadBytes(blueprints.PermissionedDisputeGame2.Bytes(), 32)...)
	result = append(result, common.LeftPadBytes(blueprints.PermissionlessDisputeGame1.Bytes(), 32)...)
	result = append(result, common.LeftPadBytes(blueprints.PermissionlessDisputeGame2.Bytes(), 32)...)

	// implementations
	result = append(result, common.LeftPadBytes(impls.SuperchainConfigImpl.Bytes(), 32)...)
	result = append(result, common.LeftPadBytes(impls.ProtocolVersionsImpl.Bytes(), 32)...)
	result = append(result, common.LeftPadBytes(impls.L1ERC721BridgeImpl.Bytes(), 32)...)
	result = append(result, common.LeftPadBytes(impls.OptimismPortalImpl.Bytes(), 32)...)
	result = append(result, common.LeftPadBytes(impls.SystemConfigImpl.Bytes(), 32)...)
	result = append(result, common.LeftPadBytes(impls.OptimismMintableERC20FactoryImpl.Bytes(), 32)...)
	result = append(result, common.LeftPadBytes(impls.L1CrossDomainMessengerImpl.Bytes(), 32)...)
	result = append(result, common.LeftPadBytes(impls.L1StandardBridgeImpl.Bytes(), 32)...)
	result = append(result, common.LeftPadBytes(impls.DisputeGameFactoryImpl.Bytes(), 32)...)
	result = append(result, common.LeftPadBytes(impls.AnchorStateRegistryImpl.Bytes(), 32)...)
	result = append(result, common.LeftPadBytes(impls.DelayedWETHImpl.Bytes(), 32)...)
	result = append(result, common.LeftPadBytes(impls.MipsImpl.Bytes(), 32)...)

	// upgrade controller
	result = append(result, common.LeftPadBytes(upgradeController.Bytes(), 32)...)

	// l1ContractsRelease (dynamic args appended to the end)
	releaseBytes := []byte(release)
	result = append(result, common.LeftPadBytes(big.NewInt(int64(len(releaseBytes))).Bytes(), 32)...)
	result = append(result, common.RightPadBytes(releaseBytes, (len(releaseBytes)+31)/32*32)...)

	return strings.TrimPrefix(hexutil.Encode(result), "0x"), nil
}

func encodeSuperchainConfigProxyArgs(v *Verifier) (string, error) {
	addr := v.st.SuperchainDeployment.ProxyAdminAddress
	padded := common.LeftPadBytes(addr.Bytes(), 32)
	return strings.TrimPrefix(hexutil.Encode(padded), "0x"), nil
}

func encodePermissionedDisputeGameArgs(v *Verifier) (string, error) {
	chainState, err := v.st.Chain(v.l2ChainID)
	if err != nil {
		return "", err
	}
	addr := chainState.PermissionedDisputeGameAddress
	result := []byte{}

	var gameType uint32
	gameTypeFn := w3.MustNewFunc("gameType()", "uint32")
	if err := v.w3Client.Call(eth.CallFunc(addr, gameTypeFn).Returns(&gameType)); err != nil {
		return "", err
	}
	result = append(result, common.LeftPadBytes(big.NewInt(int64(gameType)).Bytes(), 32)...)

	var absolutePrestate [32]byte
	absolutePrestateFn := w3.MustNewFunc("absolutePrestate()", "bytes32")
	if err := v.w3Client.Call(eth.CallFunc(addr, absolutePrestateFn).Returns(&absolutePrestate)); err != nil {
		return "", err
	}
	result = append(result, absolutePrestate[:]...)

	var maxGameDepth big.Int
	maxGameDepthFn := w3.MustNewFunc("maxGameDepth()", "uint256")
	if err := v.w3Client.Call(eth.CallFunc(addr, maxGameDepthFn).Returns(&maxGameDepth)); err != nil {
		return "", err
	}
	result = append(result, common.LeftPadBytes(maxGameDepth.Bytes(), 32)...)

	var splitDepth big.Int
	splitDepthFn := w3.MustNewFunc("splitDepth()", "uint256")
	if err := v.w3Client.Call(eth.CallFunc(addr, splitDepthFn).Returns(&splitDepth)); err != nil {
		return "", err
	}
	result = append(result, common.LeftPadBytes(splitDepth.Bytes(), 32)...)

	var clockExtension uint64
	clockExtensionFn := w3.MustNewFunc("clockExtension()", "uint64")
	if err := v.w3Client.Call(eth.CallFunc(addr, clockExtensionFn).Returns(&clockExtension)); err != nil {
		return "", err
	}
	result = append(result, common.LeftPadBytes(big.NewInt(int64(clockExtension)).Bytes(), 32)...)

	var maxClockDuration uint64
	maxClockDurationFn := w3.MustNewFunc("maxClockDuration()", "uint64")
	if err := v.w3Client.Call(eth.CallFunc(addr, maxClockDurationFn).Returns(&maxClockDuration)); err != nil {
		return "", err
	}
	result = append(result, common.LeftPadBytes(big.NewInt(int64(maxClockDuration)).Bytes(), 32)...)
	var vm common.Address
	vmFn := w3.MustNewFunc("vm()", "address")
	if err := v.w3Client.Call(eth.CallFunc(addr, vmFn).Returns(&vm)); err != nil {
		return "", err
	}
	result = append(result, common.LeftPadBytes(vm.Bytes(), 32)...)

	var weth common.Address
	wethFn := w3.MustNewFunc("weth()", "address")
	if err := v.w3Client.Call(eth.CallFunc(addr, wethFn).Returns(&weth)); err != nil {
		return "", err
	}
	result = append(result, common.LeftPadBytes(weth.Bytes(), 32)...)

	var anchorStateRegistry common.Address
	anchorStateRegistryFn := w3.MustNewFunc("anchorStateRegistry()", "address")
	if err := v.w3Client.Call(eth.CallFunc(addr, anchorStateRegistryFn).Returns(&anchorStateRegistry)); err != nil {
		return "", err
	}
	result = append(result, common.LeftPadBytes(anchorStateRegistry.Bytes(), 32)...)

	var l2ChainId big.Int
	l2ChainIdFn := w3.MustNewFunc("l2ChainId()", "uint256")
	if err := v.w3Client.Call(eth.CallFunc(addr, l2ChainIdFn).Returns(&l2ChainId)); err != nil {
		return "", err
	}
	result = append(result, common.LeftPadBytes(l2ChainId.Bytes(), 32)...)

	chainIntent, err := v.st.AppliedIntent.Chain(v.l2ChainID)
	if err != nil {
		return "", err
	}
	proposer := chainIntent.Roles.Proposer
	result = append(result, common.LeftPadBytes(proposer.Bytes(), 32)...)

	challenger := chainIntent.Roles.Challenger
	result = append(result, common.LeftPadBytes(challenger.Bytes(), 32)...)

	return strings.TrimPrefix(hexutil.Encode(result), "0x"), nil
}
