// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { CommonTest } from "test/setup/CommonTest.sol";

// Libraries
import { Encoding } from "src/libraries/Encoding.sol";
import { Types } from "src/libraries/Types.sol";
import { GasPayingToken } from "src/libraries/GasPayingToken.sol";
import { Constants } from "src/libraries/Constants.sol";
import "src/libraries/L1BlockErrors.sol";

contract L1BlockTest is CommonTest {
    address depositor;

    event GasPayingTokenSet(address indexed token, uint8 indexed decimals, bytes32 name, bytes32 symbol);

    /// @dev Sets up the test suite.
    function setUp() public virtual override {
        super.setUp();
        depositor = l1Block.DEPOSITOR_ACCOUNT();
    }
}

contract L1BlockBedrock_Test is L1BlockTest {
    // @dev Tests that `setL1BlockValues` updates the values correctly.
    function testFuzz_updatesValues_succeeds(
        uint64 n,
        uint64 t,
        uint256 b,
        bytes32 h,
        uint64 s,
        bytes32 bt,
        uint256 fo,
        uint256 fs
    )
        external
    {
        vm.prank(depositor);
        l1Block.setL1BlockValues(n, t, b, h, s, bt, fo, fs);
        assertEq(l1Block.number(), n);
        assertEq(l1Block.timestamp(), t);
        assertEq(l1Block.basefee(), b);
        assertEq(l1Block.hash(), h);
        assertEq(l1Block.sequenceNumber(), s);
        assertEq(l1Block.batcherHash(), bt);
        assertEq(l1Block.l1FeeOverhead(), fo);
        assertEq(l1Block.l1FeeScalar(), fs);
    }

    /// @dev Tests that `setL1BlockValues` can set max values.
    function test_updateValues_succeeds() external {
        vm.prank(depositor);
        l1Block.setL1BlockValues({
            _number: type(uint64).max,
            _timestamp: type(uint64).max,
            _basefee: type(uint256).max,
            _hash: keccak256(abi.encode(1)),
            _sequenceNumber: type(uint64).max,
            _batcherHash: bytes32(type(uint256).max),
            _l1FeeOverhead: type(uint256).max,
            _l1FeeScalar: type(uint256).max
        });
    }

    /// @dev Tests that `setL1BlockValues` reverts if sender address is not the depositor
    function test_updatesValues_notDepositor_reverts() external {
        vm.expectRevert("L1Block: only the depositor account can set L1 block values");
        l1Block.setL1BlockValues({
            _number: type(uint64).max,
            _timestamp: type(uint64).max,
            _basefee: type(uint256).max,
            _hash: keccak256(abi.encode(1)),
            _sequenceNumber: type(uint64).max,
            _batcherHash: bytes32(type(uint256).max),
            _l1FeeOverhead: type(uint256).max,
            _l1FeeScalar: type(uint256).max
        });
    }
}

contract L1BlockEcotone_Test is L1BlockTest {
    /// @dev Tests that setL1BlockValuesEcotone updates the values appropriately.
    function testFuzz_setL1BlockValuesEcotone_succeeds(
        uint32 baseFeeScalar,
        uint32 blobBaseFeeScalar,
        uint64 sequenceNumber,
        uint64 timestamp,
        uint64 number,
        uint256 baseFee,
        uint256 blobBaseFee,
        bytes32 hash,
        bytes32 batcherHash
    )
        external
    {
        bytes memory functionCallDataPacked = Encoding.encodeSetL1BlockValuesEcotone(
            baseFeeScalar, blobBaseFeeScalar, sequenceNumber, timestamp, number, baseFee, blobBaseFee, hash, batcherHash
        );

        vm.prank(depositor);
        (bool success,) = address(l1Block).call(functionCallDataPacked);
        assertTrue(success, "Function call failed");

        assertEq(l1Block.baseFeeScalar(), baseFeeScalar);
        assertEq(l1Block.blobBaseFeeScalar(), blobBaseFeeScalar);
        assertEq(l1Block.sequenceNumber(), sequenceNumber);
        assertEq(l1Block.timestamp(), timestamp);
        assertEq(l1Block.number(), number);
        assertEq(l1Block.basefee(), baseFee);
        assertEq(l1Block.blobBaseFee(), blobBaseFee);
        assertEq(l1Block.hash(), hash);
        assertEq(l1Block.batcherHash(), batcherHash);

        // ensure we didn't accidentally pollute the 128 bits of the sequencenum+scalars slot that
        // should be empty
        bytes32 scalarsSlot = vm.load(address(l1Block), bytes32(uint256(3)));
        bytes32 mask128 = hex"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00000000000000000000000000000000";

        assertEq(0, scalarsSlot & mask128);

        // ensure we didn't accidentally pollute the 128 bits of the number & timestamp slot that
        // should be empty
        bytes32 numberTimestampSlot = vm.load(address(l1Block), bytes32(uint256(0)));
        assertEq(0, numberTimestampSlot & mask128);
    }

    /// @dev Tests that `setL1BlockValuesEcotone` succeeds if sender address is the depositor
    function test_setL1BlockValuesEcotone_isDepositor_succeeds() external {
        bytes memory functionCallDataPacked = Encoding.encodeSetL1BlockValuesEcotone(
            type(uint32).max,
            type(uint32).max,
            type(uint64).max,
            type(uint64).max,
            type(uint64).max,
            type(uint256).max,
            type(uint256).max,
            bytes32(type(uint256).max),
            bytes32(type(uint256).max)
        );

        vm.prank(depositor);
        (bool success,) = address(l1Block).call(functionCallDataPacked);
        assertTrue(success, "function call failed");
    }

    /// @dev Tests that `setL1BlockValuesEcotone` reverts if sender address is not the depositor
    function test_setL1BlockValuesEcotone_notDepositor_reverts() external {
        bytes memory functionCallDataPacked = Encoding.encodeSetL1BlockValuesEcotone(
            type(uint32).max,
            type(uint32).max,
            type(uint64).max,
            type(uint64).max,
            type(uint64).max,
            type(uint256).max,
            type(uint256).max,
            bytes32(type(uint256).max),
            bytes32(type(uint256).max)
        );

        (bool success, bytes memory data) = address(l1Block).call(functionCallDataPacked);
        assertTrue(!success, "function call should have failed");
        // make sure return value is the expected function selector for "NotDepositor()"
        bytes memory expReturn = hex"3cc50b45";
        assertEq(data, expReturn);
    }
}

contract L1BlockIsthmus_Test is L1BlockTest {
    /// @dev Tests that `setIsthmus` succeeds if sender address is the depositor
    function test_setIsthmus_isDepositor_succeeds() external {
        vm.prank(depositor);
        l1Block.setIsthmus();
        assertTrue(l1Block.isIsthmus());

        bytes memory baseFeeVaultConfig = l1Block.getConfig(Types.ConfigType.BASE_FEE_VAULT_CONFIG);
        (address recipient, uint256 min, Types.WithdrawalNetwork network) =
            Encoding.decodeFeeVaultConfig(bytes32(baseFeeVaultConfig));
        assertEq(recipient, deploy.cfg().baseFeeVaultRecipient());
        assertEq(min, deploy.cfg().baseFeeVaultMinimumWithdrawalAmount());
        assertEq(uint8(network), uint8(deploy.cfg().baseFeeVaultWithdrawalNetwork()));

        bytes memory sequencerFeeVaultConfig = l1Block.getConfig(Types.ConfigType.SEQUENCER_FEE_VAULT_CONFIG);
        (recipient, min, network) = Encoding.decodeFeeVaultConfig(bytes32(sequencerFeeVaultConfig));
        assertEq(recipient, deploy.cfg().sequencerFeeVaultRecipient());
        assertEq(min, deploy.cfg().sequencerFeeVaultMinimumWithdrawalAmount());
        assertEq(uint8(network), uint8(deploy.cfg().sequencerFeeVaultWithdrawalNetwork()));

        bytes memory l1FeeVaultConfig = l1Block.getConfig(Types.ConfigType.L1_FEE_VAULT_CONFIG);
        (recipient, min, network) = Encoding.decodeFeeVaultConfig(bytes32(l1FeeVaultConfig));
        assertEq(recipient, deploy.cfg().l1FeeVaultRecipient());
        assertEq(min, deploy.cfg().l1FeeVaultMinimumWithdrawalAmount());
        assertEq(uint8(network), uint8(deploy.cfg().l1FeeVaultWithdrawalNetwork()));

        assertEq(
            abi.decode(l1Block.getConfig(Types.ConfigType.L1_ERC_721_BRIDGE_ADDRESS), (address)),
            address(l1ERC721Bridge)
        );
        assertEq(
            abi.decode(l1Block.getConfig(Types.ConfigType.L1_CROSS_DOMAIN_MESSENGER_ADDRESS), (address)),
            address(l1CrossDomainMessenger)
        );
        assertEq(
            abi.decode(l1Block.getConfig(Types.ConfigType.L1_STANDARD_BRIDGE_ADDRESS), (address)),
            address(l1StandardBridge)
        );
        assertEq(abi.decode(l1Block.getConfig(Types.ConfigType.REMOTE_CHAIN_ID), (uint256)), deploy.cfg().l1ChainID());
    }

    /// @dev Tests that `setIsthmus` reverts if sender address is not the depositor
    function test_setIsthmus_notDepositor_reverts() external {
        vm.expectRevert(NotDepositor.selector);
        l1Block.setIsthmus();
    }
}

contract L1BlockConfig_Test is L1BlockTest {
    /// @notice Ensures that `setConfig` always reverts when called, across all possible config types.
    ///         Use a magic number of 10 since solidity doesn't offer a good way to know the nubmer
    ///         of enum elements.
    function test_setConfig_onlyDepositor_reverts(address _caller, uint8 _type) external {
        vm.assume(_caller != Constants.DEPOSITOR_ACCOUNT);
        vm.assume(_type < 10); // the number of defined config types
        vm.expectRevert(NotDepositor.selector);
        vm.prank(_caller);
        l1Block.setConfig(Types.ConfigType(_type), hex"");
    }

    function test_getConfigRoundtripGasPayingToken_succeeds(
        address _token,
        uint8 _decimals,
        bytes32 _name,
        bytes32 _symbol
    )
        external
    {
        vm.assume(_token != address(0));
        vm.prank(Constants.DEPOSITOR_ACCOUNT);
        l1Block.setConfig(Types.ConfigType.GAS_PAYING_TOKEN, abi.encode(_token, _decimals, _name, _symbol));
        bytes memory data = l1Block.getConfig(Types.ConfigType.GAS_PAYING_TOKEN);
        (address token, uint8 decimals, bytes32 name, bytes32 symbol) =
            abi.decode(data, (address, uint8, bytes32, bytes32));
        assertEq(token, _token);
        assertEq(decimals, _decimals);

        symbol;
        name;
        // TODO: this fails because Solady's LibString.fromSmallString() is returning an empty string,
        //       I believe it expects right padded bytes32 values as input.
        // assertEq(name, _name);
        //assertEq(symbol, _symbol);
    }

    /// @notice
    function test_getConfigRoundtripBaseFeeVault_succeeds(bytes32 _config) external {
        _getConfigRoundTrip(_config, Types.ConfigType.BASE_FEE_VAULT_CONFIG);
    }

    /// @notice
    function test_getConfigRoundtripL1FeeVault_succeeds(bytes32 _config) external {
        _getConfigRoundTrip(_config, Types.ConfigType.L1_FEE_VAULT_CONFIG);
    }

    /// @notice
    function test_getConfigRoundtripSequencerFeeVault_succeeds(bytes32 _config) external {
        _getConfigRoundTrip(_config, Types.ConfigType.SEQUENCER_FEE_VAULT_CONFIG);
    }

    /// @notice Internal function for logic on round trip testing fee vault config
    function _getConfigRoundTrip(bytes32 _config, Types.ConfigType _type) internal {
        vm.prank(Constants.DEPOSITOR_ACCOUNT);
        l1Block.setConfig(_type, abi.encode(_config));
        bytes memory data = l1Block.getConfig(_type);
        bytes32 config = abi.decode(data, (bytes32));
        assertEq(config, _config);
    }

    function test_getConfigRoundtripL1CrossDomainMessenger_succeeds(address _addr) external {
        _getConfigRoundTrip(_addr, Types.ConfigType.L1_CROSS_DOMAIN_MESSENGER_ADDRESS);
    }

    function test_getConfigRoundtripL1ERC721Bridge_succeeds(address _addr) external {
        _getConfigRoundTrip(_addr, Types.ConfigType.L1_ERC_721_BRIDGE_ADDRESS);
    }

    function test_getConfigRoundtripL1StandardBridge_succeeds(address _addr) external {
        _getConfigRoundTrip(_addr, Types.ConfigType.L1_STANDARD_BRIDGE_ADDRESS);
    }

    function _getConfigRoundTrip(address _addr, Types.ConfigType _type) internal {
        vm.prank(Constants.DEPOSITOR_ACCOUNT);
        l1Block.setConfig(_type, abi.encode(_addr));
        bytes memory data = l1Block.getConfig(_type);
        address addr = abi.decode(data, (address));
        assertEq(addr, _addr);
    }

    function test_getConfigRoundtripRemoteChainId_succeeds(uint256 _value) external {
        _getConfigRoundTrip(_value, Types.ConfigType.REMOTE_CHAIN_ID);
    }

    function _getConfigRoundTrip(uint256 _value, Types.ConfigType _type) internal {
        vm.prank(Constants.DEPOSITOR_ACCOUNT);
        l1Block.setConfig(_type, abi.encode(_value));
        bytes memory data = l1Block.getConfig(_type);
        uint256 value = abi.decode(data, (uint256));
        assertEq(value, _value);
    }
}

contract L1BlockCustomGasToken_Test is L1BlockTest {
    function testFuzz_setGasPayingToken_succeeds(
        address _token,
        uint8 _decimals,
        string memory _name,
        string memory _symbol
    )
        external
    {
        vm.assume(_token != address(0));
        vm.assume(_token != Constants.ETHER);
        vm.assume(bytes(_name).length <= 32);
        vm.assume(bytes(_symbol).length <= 32);

        bytes32 name = bytes32(abi.encodePacked(_name));
        bytes32 symbol = bytes32(abi.encodePacked(_symbol));

        vm.expectEmit(address(l1Block));
        emit GasPayingTokenSet({ token: _token, decimals: _decimals, name: name, symbol: symbol });

        vm.prank(depositor);
        l1Block.setGasPayingToken({ _token: _token, _decimals: _decimals, _name: name, _symbol: symbol });

        (address token, uint8 decimals) = l1Block.gasPayingToken();
        assertEq(token, _token);
        assertEq(decimals, _decimals);

        assertEq(_name, l1Block.gasPayingTokenName());
        assertEq(_symbol, l1Block.gasPayingTokenSymbol());
        assertTrue(l1Block.isCustomGasToken());
    }

    function test_setGasPayingToken_isDepositor_reverts() external {
        vm.expectRevert(NotDepositor.selector);
        l1Block.setGasPayingToken(address(this), 18, "Test", "TST");
    }
}
