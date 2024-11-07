// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Contracts
import { StandardBridge } from "src/universal/StandardBridge.sol";
import { OptimismMintableERC20 } from "src/universal/OptimismMintableERC20.sol";

// Libraries
import { Predeploys } from "src/libraries/Predeploys.sol";

// Interfaces
import { ISemver } from "src/universal/interfaces/ISemver.sol";
import { ICrossDomainMessenger } from "src/universal/interfaces/ICrossDomainMessenger.sol";
import { IL1Block } from "src/L2/interfaces/IL1Block.sol";

/// @custom:proxied true
/// @custom:predeploy 0x4200000000000000000000000000000000000010
/// @title L2StandardBridge
/// @notice The L2StandardBridge is responsible for transfering ETH and ERC20 tokens between L1 and
///         L2. In the case that an ERC20 token is native to L2, it will be escrowed within this
///         contract. If the ERC20 token is native to L1, it will be burnt.
///         NOTE: this contract is not intended to support all variations of ERC20 tokens. Examples
///         of some token types that may not be properly supported by this contract include, but are
///         not limited to: tokens with transfer fees, rebasing tokens, and tokens with blocklists.
contract L2StandardBridge is StandardBridge, ISemver {
    /// @custom:legacy
    /// @notice Emitted whenever a withdrawal from L2 to L1 is initiated.
    /// @param l1Token   Address of the token on L1.
    /// @param l2Token   Address of the corresponding token on L2.
    /// @param from      Address of the withdrawer.
    /// @param to        Address of the recipient on L1.
    /// @param amount    Amount of the ERC20 withdrawn.
    /// @param extraData Extra data attached to the withdrawal.
    event WithdrawalInitiated(
        address indexed l1Token,
        address indexed l2Token,
        address indexed from,
        address to,
        uint256 amount,
        bytes extraData
    );

    /// @custom:legacy
    /// @notice Emitted whenever an ERC20 deposit is finalized.
    /// @param l1Token   Address of the token on L1.
    /// @param l2Token   Address of the corresponding token on L2.
    /// @param from      Address of the depositor.
    /// @param to        Address of the recipient on L2.
    /// @param amount    Amount of the ERC20 deposited.
    /// @param extraData Extra data attached to the deposit.
    event DepositFinalized(
        address indexed l1Token,
        address indexed l2Token,
        address indexed from,
        address to,
        uint256 amount,
        bytes extraData
    );

    /// @notice Semantic version.
    /// @custom:semver 1.11.1-beta.3
    function version() public pure virtual returns (string memory) {
        return "1.11.1-beta.3";
    }

    /// @notice Constructs the L2StandardBridge contract.
    constructor() StandardBridge() {
        initialize({ _otherBridge: StandardBridge(payable(address(0))) });
    }

    /// @notice Initializer.
    /// @param _otherBridge Contract for the corresponding bridge on the other chain.
    function initialize(StandardBridge _otherBridge) public initializer {
        __StandardBridge_init({
            _messenger: ICrossDomainMessenger(Predeploys.L2_CROSS_DOMAIN_MESSENGER),
            _otherBridge: _otherBridge
        });
    }

    /// @notice Allows EOAs to bridge ETH by sending directly to the bridge.
    receive() external payable override onlyEOA {
        _initiateWithdrawal(
            Predeploys.LEGACY_ERC20_ETH, msg.sender, msg.sender, msg.value, RECEIVE_DEFAULT_GAS_LIMIT, bytes("")
        );
    }

    /// @inheritdoc StandardBridge
    function gasPayingToken() internal view override returns (address addr_, uint8 decimals_) {
        (addr_, decimals_) = IL1Block(Predeploys.L1_BLOCK_ATTRIBUTES).gasPayingToken();
    }

    /// @custom:legacy
    /// @notice Initiates a withdrawal from L2 to L1.
    ///         This function only works with OptimismMintableERC20 tokens or ether. Use the
    ///         `bridgeERC20` function to bridge native L2 tokens to L1.
    ///         Subject to be deprecated in the future.
    /// @param _l2Token     Address of the L2 token to withdraw.
    /// @param _amount      Amount of the L2 token to withdraw.
    /// @param _minGasLimit Minimum gas limit to use for the transaction.
    /// @param _extraData   Extra data attached to the withdrawal.
    function withdraw(
        address _l2Token,
        uint256 _amount,
        uint32 _minGasLimit,
        bytes calldata _extraData
    )
        external
        payable
        virtual
        onlyEOA
    {
        require(isCustomGasToken() == false, "L2StandardBridge: not supported with custom gas token");
        _initiateWithdrawal(_l2Token, msg.sender, msg.sender, _amount, _minGasLimit, _extraData);
    }

    /// @custom:legacy
    /// @notice Initiates a withdrawal from L2 to L1 to a target account on L1.
    ///         Note that if ETH is sent to a contract on L1 and the call fails, then that ETH will
    ///         be locked in the L1StandardBridge. ETH may be recoverable if the call can be
    ///         successfully replayed by increasing the amount of gas supplied to the call. If the
    ///         call will fail for any amount of gas, then the ETH will be locked permanently.
    ///         This function only works with OptimismMintableERC20 tokens or ether. Use the
    ///         `bridgeERC20To` function to bridge native L2 tokens to L1.
    ///         Subject to be deprecated in the future.
    /// @param _l2Token     Address of the L2 token to withdraw.
    /// @param _to          Recipient account on L1.
    /// @param _amount      Amount of the L2 token to withdraw.
    /// @param _minGasLimit Minimum gas limit to use for the transaction.
    /// @param _extraData   Extra data attached to the withdrawal.
    function withdrawTo(
        address _l2Token,
        address _to,
        uint256 _amount,
        uint32 _minGasLimit,
        bytes calldata _extraData
    )
        external
        payable
        virtual
    {
        require(isCustomGasToken() == false, "L2StandardBridge: not supported with custom gas token");
        _initiateWithdrawal(_l2Token, msg.sender, _to, _amount, _minGasLimit, _extraData);
    }

    /// @custom:legacy
    /// @notice Retrieves the access of the corresponding L1 bridge contract.
    /// @return Address of the corresponding L1 bridge contract.
    function l1TokenBridge() external view returns (address) {
        return address(otherBridge);
    }

    /// @custom:legacy
    /// @notice Internal function to initiate a withdrawal from L2 to L1 to a target account on L1.
    /// @param _l2Token     Address of the L2 token to withdraw.
    /// @param _from        Address of the withdrawer.
    /// @param _to          Recipient account on L1.
    /// @param _amount      Amount of the L2 token to withdraw.
    /// @param _minGasLimit Minimum gas limit to use for the transaction.
    /// @param _extraData   Extra data attached to the withdrawal.
    function _initiateWithdrawal(
        address _l2Token,
        address _from,
        address _to,
        uint256 _amount,
        uint32 _minGasLimit,
        bytes memory _extraData
    )
        internal
    {
        if (_l2Token == Predeploys.LEGACY_ERC20_ETH) {
            _initiateBridgeETH(_from, _to, _amount, _minGasLimit, _extraData);
        } else {
            address l1Token = OptimismMintableERC20(_l2Token).l1Token();
            _initiateBridgeERC20(_l2Token, l1Token, _from, _to, _amount, _minGasLimit, _extraData);
        }
    }

    /// @notice Emits the legacy WithdrawalInitiated event followed by the ETHBridgeInitiated event.
    ///         This is necessary for backwards compatibility with the legacy bridge.
    /// @inheritdoc StandardBridge
    function _emitETHBridgeInitiated(
        address _from,
        address _to,
        uint256 _amount,
        bytes memory _extraData
    )
        internal
        override
    {
        emit WithdrawalInitiated(address(0), Predeploys.LEGACY_ERC20_ETH, _from, _to, _amount, _extraData);
        super._emitETHBridgeInitiated(_from, _to, _amount, _extraData);
    }

    /// @notice Emits the legacy DepositFinalized event followed by the ETHBridgeFinalized event.
    ///         This is necessary for backwards compatibility with the legacy bridge.
    /// @inheritdoc StandardBridge
    function _emitETHBridgeFinalized(
        address _from,
        address _to,
        uint256 _amount,
        bytes memory _extraData
    )
        internal
        override
    {
        emit DepositFinalized(address(0), Predeploys.LEGACY_ERC20_ETH, _from, _to, _amount, _extraData);
        super._emitETHBridgeFinalized(_from, _to, _amount, _extraData);
    }

    /// @notice Emits the legacy WithdrawalInitiated event followed by the ERC20BridgeInitiated
    ///         event. This is necessary for backwards compatibility with the legacy bridge.
    /// @inheritdoc StandardBridge
    function _emitERC20BridgeInitiated(
        address _localToken,
        address _remoteToken,
        address _from,
        address _to,
        uint256 _amount,
        bytes memory _extraData
    )
        internal
        override
    {
        emit WithdrawalInitiated(_remoteToken, _localToken, _from, _to, _amount, _extraData);
        super._emitERC20BridgeInitiated(_localToken, _remoteToken, _from, _to, _amount, _extraData);
    }

    /// @notice Emits the legacy DepositFinalized event followed by the ERC20BridgeFinalized event.
    ///         This is necessary for backwards compatibility with the legacy bridge.
    /// @inheritdoc StandardBridge
    function _emitERC20BridgeFinalized(
        address _localToken,
        address _remoteToken,
        address _from,
        address _to,
        uint256 _amount,
        bytes memory _extraData
    )
        internal
        override
    {
        emit DepositFinalized(_remoteToken, _localToken, _from, _to, _amount, _extraData);
        super._emitERC20BridgeFinalized(_localToken, _remoteToken, _from, _to, _amount, _extraData);
    }
}
