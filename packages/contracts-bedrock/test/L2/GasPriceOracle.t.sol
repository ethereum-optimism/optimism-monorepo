// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing utilities
import { CommonTest } from "test/setup/CommonTest.sol";
import { Fork } from "scripts/libraries/Config.sol";

// Libraries
import { Encoding } from "src/libraries/Encoding.sol";

contract GasPriceOracle_Test is CommonTest {
    event OverheadUpdated(uint256);
    event ScalarUpdated(uint256);
    event DecimalsUpdated(uint256);

    address depositor;

    // The initial L1 context values
    uint64 constant number = 10;
    uint64 constant timestamp = 11;
    uint256 constant baseFee = 2 * (10 ** 6);
    uint256 constant blobBaseFee = 3 * (10 ** 6);
    bytes32 constant hash = bytes32(uint256(64));
    uint64 constant sequenceNumber = 0;
    bytes32 constant batcherHash = bytes32(uint256(777));
    uint256 constant l1FeeOverhead = 310;
    uint256 constant l1FeeScalar = 10;
    uint32 constant blobBaseFeeScalar = 15;
    uint32 constant baseFeeScalar = 20;

    /// @dev Sets up the test suite.
    function setUp() public virtual override {
        super.setUp();
        depositor = l1Block.DEPOSITOR_ACCOUNT();
    }
}

contract GasPriceOracleBedrock_Test is GasPriceOracle_Test {
    /// @dev Sets up the test suite.
    function setUp() public virtual override {
        // The gasPriceOracle tests rely on an L2 genesis that is not past Ecotone.
        l2Fork = Fork.DELTA;
        super.setUp();
        assertEq(gasPriceOracle.isEcotone(), false);

        vm.prank(depositor);
        l1Block.setL1BlockValues({
            _number: number,
            _timestamp: timestamp,
            _basefee: baseFee,
            _hash: hash,
            _sequenceNumber: sequenceNumber,
            _batcherHash: batcherHash,
            _l1FeeOverhead: l1FeeOverhead,
            _l1FeeScalar: l1FeeScalar
        });
    }

    /// @dev Tests that `l1BaseFee` is set correctly.
    function test_l1BaseFee_succeeds() external view {
        assertEq(gasPriceOracle.l1BaseFee(), baseFee);
    }

    /// @dev Tests that `gasPrice` is set correctly.
    function test_gasPrice_succeeds() external {
        vm.fee(100);
        uint256 gasPrice = gasPriceOracle.gasPrice();
        assertEq(gasPrice, 100);
    }

    /// @dev Tests that `baseFee` is set correctly.
    function test_baseFee_succeeds() external {
        vm.fee(64);
        uint256 gasPrice = gasPriceOracle.baseFee();
        assertEq(gasPrice, 64);
    }

    /// @dev Tests that `scalar` is set correctly.
    function test_scalar_succeeds() external view {
        assertEq(gasPriceOracle.scalar(), l1FeeScalar);
    }

    /// @dev Tests that `overhead` is set correctly.
    function test_overhead_succeeds() external view {
        assertEq(gasPriceOracle.overhead(), l1FeeOverhead);
    }

    /// @dev Tests that `decimals` is set correctly.
    function test_decimals_succeeds() external view {
        assertEq(gasPriceOracle.decimals(), 6);
        assertEq(gasPriceOracle.DECIMALS(), 6);
    }

    /// @dev Tests that `setGasPrice` reverts since it was removed in bedrock.
    function test_setGasPrice_doesNotExist_reverts() external {
        // nosemgrep: sol-style-use-abi-encodecall
        (bool success, bytes memory returndata) =
            address(gasPriceOracle).call(abi.encodeWithSignature("setGasPrice(uint256)", 1));

        assertEq(success, false);
        assertEq(returndata, hex"");
    }

    /// @dev Tests that `setL1BaseFee` reverts since it was removed in bedrock.
    function test_setL1BaseFee_doesNotExist_reverts() external {
        // nosemgrep: sol-style-use-abi-encodecall
        (bool success, bytes memory returndata) =
            address(gasPriceOracle).call(abi.encodeWithSignature("setL1BaseFee(uint256)", 1));

        assertEq(success, false);
        assertEq(returndata, hex"");
    }

    /// @dev Tests that Fjord cannot be activated without activating Ecotone
    function test_setFjord_withoutEcotone_reverts() external {
        vm.prank(depositor);
        vm.expectRevert("GasPriceOracle: Fjord can only be activated after Ecotone");
        gasPriceOracle.setFjord();
    }
}

contract GasPriceOracleEcotone_Test is GasPriceOracle_Test {
    /// @dev Sets up the test suite.
    function setUp() public virtual override {
        l2Fork = Fork.ECOTONE;
        super.setUp();
        assertEq(gasPriceOracle.isEcotone(), true);

        bytes memory calldataPacked = Encoding.encodeSetL1BlockValuesEcotone(
            baseFeeScalar, blobBaseFeeScalar, sequenceNumber, timestamp, number, baseFee, blobBaseFee, hash, batcherHash
        );

        // Execute the function call
        vm.prank(depositor);
        (bool success,) = address(l1Block).call(calldataPacked);
        require(success, "GasPriceOracleEcotone_Test: Function call failed");
    }

    /// @dev Tests that `setEcotone` is only callable by the depositor.
    function test_setEcotone_wrongCaller_reverts() external {
        vm.expectRevert("GasPriceOracle: only the depositor account can set isEcotone flag");
        gasPriceOracle.setEcotone();
    }

    /// @dev Tests that `gasPrice` is set correctly.
    function test_gasPrice_succeeds() external {
        vm.fee(100);
        uint256 gasPrice = gasPriceOracle.gasPrice();
        assertEq(gasPrice, 100);
    }

    /// @dev Tests that `baseFee` is set correctly.
    function test_baseFee_succeeds() external {
        vm.fee(64);
        uint256 gasPrice = gasPriceOracle.baseFee();
        assertEq(gasPrice, 64);
    }

    /// @dev Tests that `overhead` reverts since it was removed in ecotone.
    function test_overhead_legacyFunction_reverts() external {
        vm.expectRevert("GasPriceOracle: overhead() is deprecated");
        gasPriceOracle.overhead();
    }

    /// @dev Tests that `scalar` reverts since it was removed in ecotone.
    function test_scalar_legacyFunction_reverts() external {
        vm.expectRevert("GasPriceOracle: scalar() is deprecated");
        gasPriceOracle.scalar();
    }

    /// @dev Tests that `l1BaseFee` is set correctly.
    function test_l1BaseFee_succeeds() external view {
        assertEq(gasPriceOracle.l1BaseFee(), baseFee);
    }

    /// @dev Tests that `blobBaseFee` is set correctly.
    function test_blobBaseFee_succeeds() external view {
        assertEq(gasPriceOracle.blobBaseFee(), blobBaseFee);
    }

    /// @dev Tests that `baseFeeScalar` is set correctly.
    function test_baseFeeScalar_succeeds() external view {
        assertEq(gasPriceOracle.baseFeeScalar(), baseFeeScalar);
    }

    /// @dev Tests that `blobBaseFeeScalar` is set correctly.
    function test_blobBaseFeeScalar_succeeds() external view {
        assertEq(gasPriceOracle.blobBaseFeeScalar(), blobBaseFeeScalar);
    }

    /// @dev Tests that `decimals` is set correctly.
    function test_decimals_succeeds() external view {
        assertEq(gasPriceOracle.decimals(), 6);
        assertEq(gasPriceOracle.DECIMALS(), 6);
    }

    /// @dev Tests that `getL1GasUsed` and `getL1Fee` return expected values
    function test_getL1Fee_succeeds() external view {
        bytes memory data = hex"0000010203"; // 2 zero bytes, 3 non-zero bytes
        // (2*4) + (3*16) + (68*16) == 1144
        uint256 gas = gasPriceOracle.getL1GasUsed(data);
        assertEq(gas, 1144);
        uint256 price = gasPriceOracle.getL1Fee(data);
        // gas * (2M*16*20 + 3M*15) / 16M == 48977.5
        assertEq(price, 48977);
    }

    /// @dev Tests that `setFjord` is only callable by the depositor.
    function test_setFjord_wrongCaller_reverts() external {
        vm.expectRevert("GasPriceOracle: only the depositor account can set isFjord flag");
        gasPriceOracle.setFjord();
    }
}

contract GasPriceOracleFjordActive_Test is GasPriceOracle_Test {
    /// @dev Sets up the test suite.
    function setUp() public virtual override {
        l2Fork = Fork.FJORD;
        super.setUp();

        bytes memory calldataPacked = Encoding.encodeSetL1BlockValuesEcotone(
            baseFeeScalar, blobBaseFeeScalar, sequenceNumber, timestamp, number, baseFee, blobBaseFee, hash, batcherHash
        );

        vm.prank(depositor);
        (bool success,) = address(l1Block).call(calldataPacked);
        require(success, "GasPriceOracleFjordActive_Test: Function call failed");
    }

    /// @dev Tests that `setFjord` cannot be called when Fjord is already activate
    function test_setFjord_whenFjordActive_reverts() external {
        vm.expectRevert("GasPriceOracle: Fjord already active");
        vm.prank(depositor);
        gasPriceOracle.setFjord();
    }

    /// @dev Tests that `gasPrice` is set correctly.
    function test_gasPrice_succeeds() external {
        vm.fee(100);
        uint256 gasPrice = gasPriceOracle.gasPrice();
        assertEq(gasPrice, 100);
    }

    /// @dev Tests that `baseFee` is set correctly.
    function test_baseFee_succeeds() external {
        vm.fee(64);
        uint256 gasPrice = gasPriceOracle.baseFee();
        assertEq(gasPrice, 64);
    }

    /// @dev Tests that `overhead` reverts since it was removed in ecotone.
    function test_overhead_legacyFunction_reverts() external {
        vm.expectRevert("GasPriceOracle: overhead() is deprecated");
        gasPriceOracle.overhead();
    }

    /// @dev Tests that `scalar` reverts since it was removed in ecotone.
    function test_scalar_legacyFunction_reverts() external {
        vm.expectRevert("GasPriceOracle: scalar() is deprecated");
        gasPriceOracle.scalar();
    }

    /// @dev Tests that `l1BaseFee` is set correctly.
    function test_l1BaseFee_succeeds() external view {
        assertEq(gasPriceOracle.l1BaseFee(), baseFee);
    }

    /// @dev Tests that `blobBaseFee` is set correctly.
    function test_blobBaseFee_succeeds() external view {
        assertEq(gasPriceOracle.blobBaseFee(), blobBaseFee);
    }

    /// @dev Tests that `baseFeeScalar` is set correctly.
    function test_baseFeeScalar_succeeds() external view {
        assertEq(gasPriceOracle.baseFeeScalar(), baseFeeScalar);
    }

    /// @dev Tests that `blobBaseFeeScalar` is set correctly.
    function test_blobBaseFeeScalar_succeeds() external view {
        assertEq(gasPriceOracle.blobBaseFeeScalar(), blobBaseFeeScalar);
    }

    /// @dev Tests that `decimals` is set correctly.
    function test_decimals_succeeds() external view {
        assertEq(gasPriceOracle.decimals(), 6);
        assertEq(gasPriceOracle.DECIMALS(), 6);
    }

    /// @dev Tests that `getL1GasUsed`, `getL1Fee` and `getL1FeeUpperBound` return expected values
    ///      for the minimum bound of the linear regression
    function test_getL1FeeMinimumBound_succeeds() external view {
        bytes memory data = hex"0000010203"; // fastlzSize: 74, inc signature
        uint256 gas = gasPriceOracle.getL1GasUsed(data);
        assertEq(gas, 1600); // 100 (minimum size) * 16
        uint256 price = gasPriceOracle.getL1Fee(data);
        // linearRegression = -42.5856 + 74 * 0.8365 = 19.3154
        // under the minTxSize of 100, so linear regression output is ignored
        // 100_000_000 * (20 * 16 * 2 * 1e6 + 3 * 1e6 * 15) / 1e12
        assertEq(price, 68500);

        assertEq(data.length, 5);
        // flzUpperBound = (5 + 68) + ((5 + 68) / 255) + 16 = 89
        // linearRegression = -42.5856 + 89 * 0.8365 = 31.8629
        // under the minTxSize of 100, so output is ignored
        // 100_000_000 * (20 * 16 * 2 * 1e6 + 3 * 1e6 * 15) / 1e12
        uint256 upperBound = gasPriceOracle.getL1FeeUpperBound(data.length);
        assertEq(upperBound, 68500);
    }

    /// @dev Tests that `getL1GasUsed`, `getL1Fee` and `getL1FeeUpperBound` return expected values
    ///      for a specific test transaction
    function test_getL1FeeRegression_succeeds() external view {
        // fastlzSize: 235, inc signature
        bytes memory data =
            hex"1d2c3ec4f5a9b3f3cd2c024e455c1143a74bbd637c324adcbd4f74e346786ac44e23e78f47d932abedd8d1"
            hex"06daadcea350be16478461046273101034601364012364701331dfad43729dc486abd134bcad61b34d6ca1"
            hex"f2eb31655b7d61ca33ba6d172cdf7d8b5b0ef389a314ca7a9a831c09fc2ca9090d059b4dd25194f3de297b"
            hex"dba6d6d796e4f80be94f8a9151d685607826e7ba25177b40cb127ea9f1438470";

        uint256 gas = gasPriceOracle.getL1GasUsed(data);
        assertEq(gas, 2463); // 235 * 16
        uint256 price = gasPriceOracle.getL1Fee(data);
        // linearRegression = -42.5856 + 235 * 0.8365 = 153.9919
        // 153_991_900 * (20 * 16 * 2 * 1e6 + 3 * 1e6 * 15) / 1e12
        assertEq(price, 105484);

        assertEq(data.length, 161);
        // flzUpperBound = (161 + 68) + ((161 + 68) / 255) + 16 = 245
        // linearRegression = -42.5856 + 245 * 0.8365 = 162.3569
        // 162_356_900 * (20 * 16 * 2 * 1e6 + 3 * 1e6 * 15) / 1e12 == 111,214.4765
        uint256 upperBound = gasPriceOracle.getL1FeeUpperBound(data.length);
        assertEq(upperBound, 111214);
    }
}
