// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Test } from "forge-std/Test.sol";
import { L2GenesisInput, L2Genesis } from "scripts/L2Genesis.s.sol";
import { Fork } from "scripts/libraries/Config.sol";
import { Predeploys } from "src/libraries/Predeploys.sol";
import { Ownable } from "@openzeppelin/contracts/access/Ownable.sol";
import { IProxy } from "interfaces/universal/IProxy.sol";
import { IL2CrossDomainMessenger } from "interfaces/L2/IL2CrossDomainMessenger.sol";
import { IL2StandardBridge } from "interfaces/L2/IL2StandardBridge.sol";
import { IL2ERC721Bridge } from "interfaces/L2/IL2ERC721Bridge.sol";
import { ISequencerFeeVault } from "interfaces/L2/ISequencerFeeVault.sol";
import { IBaseFeeVault } from "interfaces/L2/IBaseFeeVault.sol";
import { IL1FeeVault } from "interfaces/L2/IL1FeeVault.sol";

contract L2GenesisTest is Test {
    L2GenesisInput internal l2i;

    L2Genesis internal genesis;

    function setUp() public {
        l2i = new L2GenesisInput();
        l2i.set(l2i.l1CrossDomainMessengerProxy.selector, makeAddr("l1CrossDomainMessengerProxy"));
        l2i.set(l2i.l1StandardBridgeProxy.selector, makeAddr("l1StandardBridgeProxy"));
        l2i.set(l2i.l1ERC721BridgeProxy.selector, makeAddr("l1ERC721BridgeProxy"));
        l2i.set(l2i.fundDevAccounts.selector, false);
        l2i.set(l2i.useInterop.selector, false);
        l2i.set(l2i.fork.selector, Fork.ECOTONE);
        l2i.set(l2i.l1ChainId.selector, 1);
        l2i.set(l2i.l2ChainId.selector, 999);
        l2i.set(l2i.proxyAdminOwner.selector, makeAddr("proxyAdminOwner"));
        l2i.set(l2i.sequencerFeeVaultRecipient.selector, makeAddr("sequencerFeeVaultRecipient"));
        l2i.set(l2i.sequencerFeeVaultMinimumWithdrawalAmount.selector, 1);
        l2i.set(l2i.sequencerFeeVaultWithdrawalNetwork.selector, 1);
        l2i.set(l2i.baseFeeVaultRecipient.selector, makeAddr("baseFeeVaultRecipient"));
        l2i.set(l2i.baseFeeVaultMinimumWithdrawalAmount.selector, 2);
        l2i.set(l2i.baseFeeVaultWithdrawalNetwork.selector, 1);
        l2i.set(l2i.l1FeeVaultRecipient.selector, makeAddr("l1FeeVaultRecipient"));
        l2i.set(l2i.l1FeeVaultMinimumWithdrawalAmount.selector, 3);
        l2i.set(l2i.l1FeeVaultWithdrawalNetwork.selector, 0);
        l2i.set(l2i.enableGovernance.selector, true);
        l2i.set(l2i.governanceTokenOwner.selector, makeAddr("governanceTokenOwner"));

        genesis = new L2Genesis();
    }

    function test_run_noInteropNoDev_works() public {
        genesis.run(l2i);

        for (uint256 i = 0; i < genesis.PRECOMPILE_COUNT(); i++) {
            assertEq(address(uint160(i)).balance, 1);
        }

        for (uint256 i = 0; i < 30; i++) {
            assertEq(genesis.devAccounts(i).balance, 0);
        }

        address l1CrossDomainMessenger =
            address(IL2CrossDomainMessenger(Predeploys.L2_CROSS_DOMAIN_MESSENGER).l1CrossDomainMessenger());
        assertEq(l1CrossDomainMessenger, l2i.l1CrossDomainMessengerProxy());
        address otherBridge = address(IL2StandardBridge(payable(Predeploys.L2_STANDARD_BRIDGE)).otherBridge());
        assertEq(otherBridge, l2i.l1StandardBridgeProxy());
        otherBridge = address(IL2ERC721Bridge(Predeploys.L2_ERC721_BRIDGE).OTHER_BRIDGE());
        assertEq(otherBridge, l2i.l1ERC721BridgeProxy());

        ISequencerFeeVault sfVault = ISequencerFeeVault(payable(Predeploys.SEQUENCER_FEE_WALLET));
        assertEq(sfVault.recipient(), l2i.sequencerFeeVaultRecipient());
        assertEq(sfVault.minWithdrawalAmount(), l2i.sequencerFeeVaultMinimumWithdrawalAmount());
        assertEq(uint256(sfVault.withdrawalNetwork()), l2i.sequencerFeeVaultWithdrawalNetwork());

        IBaseFeeVault bfVault = IBaseFeeVault(payable(Predeploys.BASE_FEE_VAULT));
        assertEq(bfVault.recipient(), l2i.baseFeeVaultRecipient());
        assertEq(bfVault.minWithdrawalAmount(), l2i.baseFeeVaultMinimumWithdrawalAmount());
        assertEq(uint256(bfVault.withdrawalNetwork()), l2i.baseFeeVaultWithdrawalNetwork());

        IL1FeeVault l1Vault = IL1FeeVault(payable(Predeploys.L1_FEE_VAULT));
        assertEq(l1Vault.recipient(), l2i.l1FeeVaultRecipient());
        assertEq(l1Vault.minWithdrawalAmount(), l2i.l1FeeVaultMinimumWithdrawalAmount());
        assertEq(uint256(l1Vault.withdrawalNetwork()), l2i.l1FeeVaultWithdrawalNetwork());

        assertGt(Predeploys.GOVERNANCE_TOKEN.code.length, 0);
        assertEq(Ownable(Predeploys.GOVERNANCE_TOKEN).owner(), l2i.governanceTokenOwner());

        assertNoImpl(Predeploys.CROSS_L2_INBOX);
        assertNoImpl(Predeploys.L2_TO_L2_CROSS_DOMAIN_MESSENGER);
        assertNoImpl(Predeploys.SUPERCHAIN_WETH);
        assertNoImpl(Predeploys.ETH_LIQUIDITY);
        assertNoImpl(Predeploys.OPTIMISM_SUPERCHAIN_ERC20_FACTORY);
        assertNoImpl(Predeploys.OPTIMISM_SUPERCHAIN_ERC20_BEACON);
        assertNoImpl(Predeploys.SUPERCHAIN_TOKEN_BRIDGE);

        assertEq(Predeploys.ETH_LIQUIDITY.balance, 0);
    }

    function test_fundDevAccounts_works() public {
        l2i.set(l2i.fundDevAccounts.selector, true);
        genesis.run(l2i);

        for (uint256 i = 0; i < 30; i++) {
            assertEq(genesis.devAccounts(i).balance, genesis.DEV_ACCOUNT_FUND_AMT());
        }
    }

    function test_run_interopNoDev_works() public {
        l2i.set(l2i.useInterop.selector, true);
        genesis.run(l2i);

        address impl = Predeploys.predeployToCodeNamespace(Predeploys.L2_STANDARD_BRIDGE);
        bytes memory expectedCode = vm.getDeployedCode("L2StandardBridgeInterop.sol:L2StandardBridgeInterop");
        assertEq(impl.code, expectedCode);

        impl = Predeploys.predeployToCodeNamespace(Predeploys.L1_BLOCK_ATTRIBUTES);
        expectedCode = vm.getDeployedCode("L1BlockInterop.sol:L1BlockInterop");
        assertEq(impl.code, expectedCode);

        // Check all interop predeploys have code
        assertHasImpl(Predeploys.CROSS_L2_INBOX);
        assertHasImpl(Predeploys.L2_TO_L2_CROSS_DOMAIN_MESSENGER);
        assertHasImpl(Predeploys.SUPERCHAIN_WETH);
        assertHasImpl(Predeploys.ETH_LIQUIDITY);
        assertHasImpl(Predeploys.OPTIMISM_SUPERCHAIN_ERC20_FACTORY);
        assertHasImpl(Predeploys.OPTIMISM_SUPERCHAIN_ERC20_BEACON);
        assertHasImpl(Predeploys.SUPERCHAIN_TOKEN_BRIDGE);

        assertEq(Predeploys.ETH_LIQUIDITY.balance, type(uint248).max);
    }

    function test_run_noGovernance_works() public {
        l2i.set(l2i.enableGovernance.selector, false);
        genesis.run(l2i);
        assertEq(Predeploys.GOVERNANCE_TOKEN.code.length, 0);
    }

    function assertNoImpl(address addr) internal {
        IProxy proxy = IProxy(payable(addr));
        vm.prank(address(0));
        address impl = proxy.implementation();
        assertEq(impl, address(0));
    }

    function assertHasImpl(address addr) internal {
        IProxy proxy = IProxy(payable(addr));
        vm.prank(address(0));
        address impl = proxy.implementation();
        assertGt(impl.code.length, 0);
    }
}
