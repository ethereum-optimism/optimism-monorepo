// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { CommonTest } from "test/setup/CommonTest.sol";

// Contracts
import { ERC20 } from "@openzeppelin/contracts/token/ERC20/ERC20.sol";

// Libraries
import { Constants } from "src/libraries/Constants.sol";
import { StaticConfig } from "src/libraries/StaticConfig.sol";
import { GasPayingToken } from "src/libraries/GasPayingToken.sol";

// Interfaces
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { ISystemConfigInterop } from "interfaces/L1/ISystemConfigInterop.sol";
import { IOptimismPortalInterop } from "interfaces/L1/IOptimismPortalInterop.sol";
import { ConfigType } from "interfaces/L2/IL1BlockInterop.sol";

contract SystemConfigInterop_Test is CommonTest {
    /// @notice Marked virtual to be overridden in
    ///         test/kontrol/deployment/DeploymentSummary.t.sol
    function setUp() public virtual override {
        super.enableInterop();
        super.setUp();
    }

    /// @dev Tests that the constructor sets the correct values.
    function test_constructor_succeeds() external view {
        ISystemConfigInterop impl = ISystemConfigInterop(payable(deploy.mustGetAddress("SystemConfig")));
        assertEq(impl.SUPERCHAIN_CONFIG(), address(superchainConfig));
        assertEq(_systemConfigInterop().SUPERCHAIN_CONFIG(), address(superchainConfig));
    }

    /// @dev Tests that when the decimals is not 18, initialization reverts.
    function test_initialize_decimalsIsNot18_reverts(uint8 decimals) external {
        vm.assume(decimals != 18);
        address _token = address(L1Token);

        vm.mockCall(_token, abi.encodeCall(ERC20.name, ()), abi.encode("Token"));
        vm.mockCall(_token, abi.encodeCall(ERC20.symbol, ()), abi.encode("TKN"));
        vm.mockCall(_token, abi.encodeCall(ERC20.decimals, ()), abi.encode(decimals));

        vm.expectRevert("SystemConfig: bad decimals of gas paying token");
        _cleanStorageAndInit(_token);
    }

    /// @dev Tests that the gas paying token can be set.
    function testFuzz_setGasPayingToken_succeeds(
        address _token,
        string calldata _name,
        string calldata _symbol
    )
        public
    {
        assumeNotForgeAddress(_token);
        vm.assume(_token != address(0));
        vm.assume(_token != Constants.ETHER);

        // Using vm.assume() would cause too many test rejections.
        string memory name = _name;
        if (bytes(_name).length > 32) {
            name = _name[:32];
        }

        // Using vm.assume() would cause too many test rejections.
        string memory symbol = _symbol;
        if (bytes(_symbol).length > 32) {
            symbol = _symbol[:32];
        }

        vm.mockCall(_token, abi.encodeCall(ERC20.decimals, ()), abi.encode(18));
        vm.mockCall(_token, abi.encodeCall(ERC20.name, ()), abi.encode(name));
        vm.mockCall(_token, abi.encodeCall(ERC20.symbol, ()), abi.encode(symbol));

        vm.expectCall(
            address(optimismPortal),
            abi.encodeCall(
                IOptimismPortalInterop.setConfig,
                (
                    ConfigType.SET_GAS_PAYING_TOKEN,
                    StaticConfig.encodeSetGasPayingToken({
                        _token: _token,
                        _decimals: 18,
                        _name: GasPayingToken.sanitize(name),
                        _symbol: GasPayingToken.sanitize(symbol)
                    })
                )
            )
        );

        _cleanStorageAndInit(_token);
    }

    /// @dev Tests that a dependency can be added.
    function testFuzz_addDependency_succeeds(uint256 _chainId) public {
        vm.expectCall(
            address(optimismPortal),
            abi.encodeCall(
                IOptimismPortalInterop.setConfig,
                (ConfigType.ADD_DEPENDENCY, StaticConfig.encodeAddDependency(_chainId))
            )
        );

        vm.prank(address(superchainConfig));
        _systemConfigInterop().addDependency(_chainId);

        assertEq(_systemConfigInterop().dependencyCounter(), 1);
    }

    /// @dev Tests that adding a dependency as not the SuperchainConfig reverts.
    function testFuzz_addDependency_notSuperchainConfig_reverts(uint256 _chainId) public {
        require(alice != address(superchainConfig), "SystemConfigInterop_Test: 100");
        vm.expectRevert("SystemConfig: caller is not the SuperchainConfig");
        vm.prank(alice);
        _systemConfigInterop().addDependency(_chainId);
    }

    /// @dev Tests that a dependency can be removed.
    function testFuzz_removeDependency_succeeds(uint256 _chainId) public {
        // Add the dependency first
        vm.prank(address(superchainConfig));
        _systemConfigInterop().addDependency(_chainId);
        assertEq(_systemConfigInterop().dependencyCounter(), 1);

        vm.expectCall(
            address(optimismPortal),
            abi.encodeCall(
                IOptimismPortalInterop.setConfig,
                (ConfigType.REMOVE_DEPENDENCY, StaticConfig.encodeRemoveDependency(_chainId))
            )
        );

        vm.prank(address(superchainConfig));
        _systemConfigInterop().removeDependency(_chainId);

        assertEq(_systemConfigInterop().dependencyCounter(), 0);
    }

    /// @dev Tests that removing a dependency as not the SuperchainConfig reverts.
    function testFuzz_removeDependency_notSuperchainConfig_reverts(uint256 _chainId) public {
        require(alice != address(superchainConfig), "SystemConfigInterop_Test: 100");
        vm.expectRevert("SystemConfig: caller is not the SuperchainConfig");
        vm.prank(alice);
        _systemConfigInterop().removeDependency(_chainId);
    }

    function test_dependencyCounter_succeeds() public {
        assertEq(_systemConfigInterop().dependencyCounter(), 0);

        // Add a dependency
        vm.prank(address(superchainConfig));
        _systemConfigInterop().addDependency(1);
        assertEq(_systemConfigInterop().dependencyCounter(), 1);

        // Remove the dependency
        vm.prank(address(superchainConfig));
        _systemConfigInterop().removeDependency(1);
        assertEq(_systemConfigInterop().dependencyCounter(), 0);
    }

    /// @dev Helper to clean storage and then initialize the system config with an arbitrary gas token address.
    function _cleanStorageAndInit(address _token) internal {
        // Wipe out the initialized slot so the proxy can be initialized again
        vm.store(address(systemConfig), bytes32(0), bytes32(0));
        vm.store(address(systemConfig), GasPayingToken.GAS_PAYING_TOKEN_SLOT, bytes32(0));
        vm.store(address(systemConfig), GasPayingToken.GAS_PAYING_TOKEN_NAME_SLOT, bytes32(0));
        vm.store(address(systemConfig), GasPayingToken.GAS_PAYING_TOKEN_SYMBOL_SLOT, bytes32(0));

        systemConfig.initialize({
            _owner: alice,
            _basefeeScalar: 2100,
            _blobbasefeeScalar: 1000000,
            _batcherHash: bytes32(hex"abcd"),
            _gasLimit: 30_000_000,
            _unsafeBlockSigner: address(1),
            _config: Constants.DEFAULT_RESOURCE_CONFIG(),
            _batchInbox: address(0),
            _addresses: ISystemConfig.Addresses({
                l1CrossDomainMessenger: address(0),
                l1ERC721Bridge: address(0),
                disputeGameFactory: address(0),
                l1StandardBridge: address(0),
                optimismPortal: address(optimismPortal),
                optimismMintableERC20Factory: address(0),
                gasPayingToken: _token
            })
        });
    }

    /// @dev Returns the SystemConfigInterop instance.
    function _systemConfigInterop() internal view returns (ISystemConfigInterop) {
        return ISystemConfigInterop(address(systemConfig));
    }
}
