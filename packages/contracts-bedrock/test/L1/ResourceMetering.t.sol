// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { Test } from "forge-std/Test.sol";

// Contracts
import { ResourceMetering } from "src/L1/ResourceMetering.sol";

// Libraries
import { Constants } from "src/libraries/Constants.sol";

// Interfaces
import { IResourceMetering } from "src/L1/interfaces/IResourceMetering.sol";

contract MeterUser is ResourceMetering {
    ResourceMetering.ResourceConfig public innerConfig;

    constructor() {
        initialize();
        IResourceMetering.ResourceConfig memory rcfg = Constants.DEFAULT_RESOURCE_CONFIG();
        innerConfig = ResourceMetering.ResourceConfig({
            maxResourceLimit: rcfg.maxResourceLimit,
            elasticityMultiplier: rcfg.elasticityMultiplier,
            baseFeeMaxChangeDenominator: rcfg.baseFeeMaxChangeDenominator,
            minimumBaseFee: rcfg.minimumBaseFee,
            systemTxMaxGas: rcfg.systemTxMaxGas,
            maximumBaseFee: rcfg.maximumBaseFee
        });
    }

    function initialize() public initializer {
        __ResourceMetering_init();
    }

    function resourceConfig() public view returns (ResourceMetering.ResourceConfig memory) {
        return _resourceConfig();
    }

    function _resourceConfig() internal view override returns (ResourceMetering.ResourceConfig memory) {
        return innerConfig;
    }

    function use(uint64 _amount) public metered(_amount) { }

    function set(uint128 _prevBaseFee, uint64 _prevBoughtGas, uint64 _prevBlockNum) public {
        params = ResourceMetering.ResourceParams({
            prevBaseFee: _prevBaseFee,
            prevBoughtGas: _prevBoughtGas,
            prevBlockNum: _prevBlockNum
        });
    }

    function setParams(ResourceMetering.ResourceConfig memory newConfig) public {
        innerConfig = newConfig;
    }
}

/// @title ResourceMetering_Test
/// @dev Tests are based on the default config values.
///      It is expected that these config values are used in production.
contract ResourceMetering_Test is Test {
    MeterUser internal meter;
    uint64 initialBlockNum;

    /// @dev Sets up the test contract.
    function setUp() public {
        meter = new MeterUser();
        initialBlockNum = uint64(block.number);
    }

    /// @dev Tests that the initial resource params are set correctly.
    function test_meter_initialResourceParams_succeeds() external view {
        (uint128 prevBaseFee, uint64 prevBoughtGas, uint64 prevBlockNum) = meter.params();
        ResourceMetering.ResourceConfig memory rcfg = meter.resourceConfig();

        assertEq(prevBaseFee, rcfg.minimumBaseFee);
        assertEq(prevBoughtGas, 0);
        assertEq(prevBlockNum, initialBlockNum);
    }

    /// @dev Tests that reinitializing the resource params are set correctly.
    function test_meter_reinitializedResourceParams_succeeds() external {
        (uint128 prevBaseFee, uint64 prevBoughtGas, uint64 prevBlockNum) = meter.params();

        // Reset the initialized slot to enable reinitialization.
        vm.store(address(meter), bytes32(uint256(0)), bytes32(uint256(0)));
        meter.initialize();

        (uint128 postBaseFee, uint64 postBoughtGas, uint64 postBlockNum) = meter.params();
        assertEq(prevBaseFee, postBaseFee);
        assertEq(prevBoughtGas, postBoughtGas);
        assertEq(prevBlockNum, postBlockNum);
    }

    /// @dev Tests that updating the resource params to the same values works correctly.
    function test_meter_updateParamsNoChange_succeeds() external {
        meter.use(0); // equivalent to just updating the base fee and block number
        (uint128 prevBaseFee, uint64 prevBoughtGas, uint64 prevBlockNum) = meter.params();
        meter.use(0);
        (uint128 postBaseFee, uint64 postBoughtGas, uint64 postBlockNum) = meter.params();

        assertEq(postBaseFee, prevBaseFee);
        assertEq(postBoughtGas, prevBoughtGas);
        assertEq(postBlockNum, prevBlockNum);
    }

    /// @dev Tests that updating the initial block number sets the meter params correctly.
    function test_meter_updateOneEmptyBlock_succeeds() external {
        vm.roll(initialBlockNum + 1);
        meter.use(0);
        (uint128 prevBaseFee, uint64 prevBoughtGas, uint64 prevBlockNum) = meter.params();

        assertEq(prevBaseFee, 1 gwei);
        assertEq(prevBoughtGas, 0);
        assertEq(prevBlockNum, initialBlockNum + 1);
    }

    /// @dev Tests that updating the initial block number sets the meter params correctly.
    function test_meter_updateTwoEmptyBlocks_succeeds() external {
        vm.roll(initialBlockNum + 2);
        meter.use(0);
        (uint128 prevBaseFee, uint64 prevBoughtGas, uint64 prevBlockNum) = meter.params();

        assertEq(prevBaseFee, 1 gwei);
        assertEq(prevBoughtGas, 0);
        assertEq(prevBlockNum, initialBlockNum + 2);
    }

    /// @dev Tests that updating the initial block number sets the meter params correctly.
    function test_meter_updateTenEmptyBlocks_succeeds() external {
        vm.roll(initialBlockNum + 10);
        meter.use(0);
        (uint128 prevBaseFee, uint64 prevBoughtGas, uint64 prevBlockNum) = meter.params();

        assertEq(prevBaseFee, 1 gwei);
        assertEq(prevBoughtGas, 0);
        assertEq(prevBlockNum, initialBlockNum + 10);
    }

    /// @dev Tests that updating the gas delta sets the meter params correctly.
    function test_meter_updateNoGasDelta_succeeds() external {
        ResourceMetering.ResourceConfig memory rcfg = meter.resourceConfig();
        uint256 target = uint256(rcfg.maxResourceLimit) / uint256(rcfg.elasticityMultiplier);
        meter.use(uint64(target));
        (uint128 prevBaseFee, uint64 prevBoughtGas, uint64 prevBlockNum) = meter.params();

        assertEq(prevBaseFee, 1000000000);
        assertEq(prevBoughtGas, target);
        assertEq(prevBlockNum, initialBlockNum);
    }

    /// @dev Tests that the meter params are set correctly for the maximum gas delta.
    function test_meter_useMax_succeeds() external {
        ResourceMetering.ResourceConfig memory rcfg = meter.resourceConfig();
        uint64 target = uint64(rcfg.maxResourceLimit) / uint64(rcfg.elasticityMultiplier);
        uint64 elasticityMultiplier = uint64(rcfg.elasticityMultiplier);

        meter.use(target * elasticityMultiplier);

        (, uint64 prevBoughtGas,) = meter.params();
        assertEq(prevBoughtGas, target * elasticityMultiplier);

        vm.roll(initialBlockNum + 1);
        meter.use(0);
        (uint128 postBaseFee,,) = meter.params();
        assertEq(postBaseFee, 2125000000);
    }

    /// @dev Tests that the metered modifier reverts if the baseFeeMaxChangeDenominator is set to 1.
    ///      Since the metered modifier internally calls solmate's powWad function, it will revert
    ///      with the error string "UNDEFINED" since the first parameter will be computed as 0.
    function test_meter_denominatorEq1_reverts() external {
        ResourceMetering.ResourceConfig memory rcfg = meter.resourceConfig();
        uint64 target = uint64(rcfg.maxResourceLimit) / uint64(rcfg.elasticityMultiplier);
        uint64 elasticityMultiplier = uint64(rcfg.elasticityMultiplier);
        rcfg.baseFeeMaxChangeDenominator = 1;
        meter.setParams(rcfg);
        meter.use(target * elasticityMultiplier);

        (, uint64 prevBoughtGas,) = meter.params();
        assertEq(prevBoughtGas, target * elasticityMultiplier);

        vm.roll(initialBlockNum + 2);

        vm.expectRevert("UNDEFINED");
        meter.use(0);
    }

    /// @dev Tests that the metered modifier reverts if the value is greater than allowed.
    function test_meter_useMoreThanMax_reverts() external {
        ResourceMetering.ResourceConfig memory rcfg = meter.resourceConfig();
        uint64 target = uint64(rcfg.maxResourceLimit) / uint64(rcfg.elasticityMultiplier);
        uint64 elasticityMultiplier = uint64(rcfg.elasticityMultiplier);

        vm.expectRevert(ResourceMetering.OutOfGas.selector);
        meter.use(target * elasticityMultiplier + 1);
    }

    /// @dev Tests that resource metering can handle large gaps between deposits.
    function testFuzz_meter_largeBlockDiff_succeeds(uint64 _amount, uint256 _blockDiff) external {
        // This test fails if the following line is commented out.
        // At 12 seconds per block, this number is effectively unreachable.
        vm.assume(_blockDiff < 433576281058164217753225238677900874458691);

        ResourceMetering.ResourceConfig memory rcfg = meter.resourceConfig();
        uint64 target = uint64(rcfg.maxResourceLimit) / uint64(rcfg.elasticityMultiplier);
        uint64 elasticityMultiplier = uint64(rcfg.elasticityMultiplier);

        vm.assume(_amount < target * elasticityMultiplier);
        vm.roll(initialBlockNum + _blockDiff);
        meter.use(_amount);
    }

    function testFuzz_meter_useGas_succeeds(uint64 _amount) external {
        (, uint64 prevBoughtGas,) = meter.params();
        vm.assume(prevBoughtGas + _amount <= meter.resourceConfig().maxResourceLimit);

        meter.use(_amount);

        (, uint64 postPrevBoughtGas,) = meter.params();
        assertEq(postPrevBoughtGas, prevBoughtGas + _amount);
    }
}

/// @title CustomMeterUser
/// @notice A simple wrapper around `ResourceMetering` that allows the initial
///         params to be set in the constructor.
contract CustomMeterUser is ResourceMetering {
    uint256 public startGas;
    uint256 public endGas;

    constructor(uint128 _prevBaseFee, uint64 _prevBoughtGas, uint64 _prevBlockNum) {
        params = ResourceMetering.ResourceParams({
            prevBaseFee: _prevBaseFee,
            prevBoughtGas: _prevBoughtGas,
            prevBlockNum: _prevBlockNum
        });
    }

    function _resourceConfig() internal pure override returns (ResourceMetering.ResourceConfig memory) {
        IResourceMetering.ResourceConfig memory rcfg = Constants.DEFAULT_RESOURCE_CONFIG();
        return ResourceMetering.ResourceConfig({
            maxResourceLimit: rcfg.maxResourceLimit,
            elasticityMultiplier: rcfg.elasticityMultiplier,
            baseFeeMaxChangeDenominator: rcfg.baseFeeMaxChangeDenominator,
            minimumBaseFee: rcfg.minimumBaseFee,
            systemTxMaxGas: rcfg.systemTxMaxGas,
            maximumBaseFee: rcfg.maximumBaseFee
        });
    }

    function use(uint64 _amount) public returns (uint256) {
        uint256 initialGas = gasleft();
        _metered(_amount, initialGas);
        return initialGas - gasleft();
    }
}

/// @title ArtifactResourceMetering_Test
/// @notice A table test that sets the state of the ResourceParams and then requests
///         various amounts of gas. This test ensures that a wide range of values
///         can safely be used with the `ResourceMetering` contract.
///         It also writes a CSV file to disk that includes useful information
///         about how much gas is used and how expensive it is in USD terms to
///         purchase the deposit gas.
contract ArtifactResourceMetering_Test is Test {
    uint128 internal minimumBaseFee;
    uint128 internal maximumBaseFee;
    uint64 internal maxResourceLimit;
    uint64 internal targetResourceLimit;

    string internal outfile;

    // keccak256(abi.encodeWithSignature("Error(string)", "ResourceMetering: cannot buy more gas than available gas
    // limit"))
    bytes32 internal cannotBuyMoreGas = 0x84edc668cfd5e050b8999f43ff87a1faaa93e5f935b20bc1dd4d3ff157ccf429;
    // keccak256(abi.encodeWithSignature("Panic(uint256)", 0x11))
    bytes32 internal overflowErr = 0x1ca389f2c8264faa4377de9ce8e14d6263ef29c68044a9272d405761bab2db27;
    // keccak256(hex"")
    bytes32 internal emptyReturnData = 0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470;

    /// @dev Sets up the tests with constants from the ResourceMetering contract.
    function setUp() public {
        vm.roll(1_000_000);

        MeterUser base = new MeterUser();
        ResourceMetering.ResourceConfig memory rcfg = base.resourceConfig();
        minimumBaseFee = uint128(rcfg.minimumBaseFee);
        maximumBaseFee = rcfg.maximumBaseFee;
        maxResourceLimit = uint64(rcfg.maxResourceLimit);
        targetResourceLimit = uint64(rcfg.maxResourceLimit) / uint64(rcfg.elasticityMultiplier);

        outfile = string.concat(vm.projectRoot(), "/.resource-metering.csv");
        try vm.removeFile(outfile) { } catch { }
    }

    /// @dev Generates a CSV file. No more than the L1 block gas limit should
    ///      be supplied to the `meter` function to avoid long execution time.
    ///      This test is skipped because there is no need to run it every time.
    ///      It generates a CSV file on disk that can be used to analyze the
    ///      gas usage and cost of the `ResourceMetering` contract. The next time
    ///      that the gas usage needs to be analyzed, the skip may be removed.
    function test_meter_generateArtifact_succeeds() external {
        vm.skip({ skipTest: true });

        vm.writeLine(
            outfile,
            "prevBaseFee,prevBoughtGas,prevBlockNumDiff,l1BaseFee,requestedGas,gasConsumed,ethPrice,usdCost,success"
        );

        // prevBaseFee value in ResourceParams
        uint128[] memory prevBaseFees = new uint128[](5);
        prevBaseFees[0] = minimumBaseFee;
        prevBaseFees[1] = maximumBaseFee;
        prevBaseFees[2] = uint128(50 gwei);
        prevBaseFees[3] = uint128(100 gwei);
        prevBaseFees[4] = uint128(200 gwei);

        // prevBoughtGas value in ResourceParams
        uint64[] memory prevBoughtGases = new uint64[](1);
        prevBoughtGases[0] = uint64(0);

        // prevBlockNum diff, simulates blocks with no deposits when non zero
        uint64[] memory prevBlockNumDiffs = new uint64[](2);
        prevBlockNumDiffs[0] = 0;
        prevBlockNumDiffs[1] = 1;

        // The amount of L2 gas that a user requests
        uint64[] memory requestedGases = new uint64[](3);
        requestedGases[0] = maxResourceLimit;
        requestedGases[1] = targetResourceLimit;
        requestedGases[2] = uint64(100_000);

        // The L1 base fee
        uint256[] memory l1BaseFees = new uint256[](4);
        l1BaseFees[0] = 1 gwei;
        l1BaseFees[1] = 50 gwei;
        l1BaseFees[2] = 75 gwei;
        l1BaseFees[3] = 100 gwei;

        // USD price of 1 ether
        uint256[] memory ethPrices = new uint256[](2);
        ethPrices[0] = 1600;
        ethPrices[1] = 3200;

        // Iterate over all of the test values and run a test
        for (uint256 i; i < prevBaseFees.length; i++) {
            for (uint256 j; j < prevBoughtGases.length; j++) {
                for (uint256 k; k < prevBlockNumDiffs.length; k++) {
                    for (uint256 l; l < requestedGases.length; l++) {
                        for (uint256 m; m < l1BaseFees.length; m++) {
                            for (uint256 n; n < ethPrices.length; n++) {
                                uint256 snapshotId = vm.snapshot();

                                uint128 prevBaseFee = prevBaseFees[i];
                                uint64 prevBoughtGas = prevBoughtGases[j];
                                uint64 prevBlockNumDiff = prevBlockNumDiffs[k];
                                uint64 requestedGas = requestedGases[l];
                                uint256 l1BaseFee = l1BaseFees[m];
                                uint256 ethPrice = ethPrices[n];
                                string memory result = "success";

                                vm.fee(l1BaseFee);

                                CustomMeterUser meter = new CustomMeterUser({
                                    _prevBaseFee: prevBaseFee,
                                    _prevBoughtGas: prevBoughtGas,
                                    _prevBlockNum: uint64(block.number)
                                });

                                vm.roll(block.number + prevBlockNumDiff);

                                // Call the metering code and catch the various
                                // types of errors.
                                uint256 gasConsumed = 0;
                                try meter.use{ gas: 30_000_000 }(requestedGas) returns (uint256 gasConsumed_) {
                                    gasConsumed = gasConsumed_;
                                } catch (bytes memory err) {
                                    bytes32 hash = keccak256(err);
                                    if (hash == cannotBuyMoreGas) {
                                        result = "ResourceMetering: cannot buy more gas than available gas limit";
                                    } else if (hash == overflowErr) {
                                        result = "arithmetic overflow/underflow";
                                    } else if (hash == emptyReturnData) {
                                        result = "out of gas";
                                    } else {
                                        result = "UNKNOWN ERROR";
                                    }
                                }

                                // Compute the USD cost of the gas used
                                uint256 usdCost = (gasConsumed * l1BaseFee * ethPrice) / 1 ether;

                                vm.writeLine(
                                    outfile,
                                    string.concat(
                                        vm.toString(prevBaseFee),
                                        ",",
                                        vm.toString(prevBoughtGas),
                                        ",",
                                        vm.toString(prevBlockNumDiff),
                                        ",",
                                        vm.toString(l1BaseFee),
                                        ",",
                                        vm.toString(requestedGas),
                                        ",",
                                        vm.toString(gasConsumed),
                                        ",",
                                        "$",
                                        vm.toString(ethPrice),
                                        ",",
                                        "$",
                                        vm.toString(usdCost),
                                        ",",
                                        result
                                    )
                                );

                                assertTrue(vm.revertTo(snapshotId));
                            }
                        }
                    }
                }
            }
        }
    }
}
