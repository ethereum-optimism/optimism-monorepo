// SPDX-License-Identifier: MIT
pragma solidity 0.8.25;

import { Test } from "forge-std/Test.sol";
import { CheckSecrets } from "src/periphery/drippie/dripchecks/CheckSecrets.sol";

/// @title  CheckSecretsTest
contract CheckSecretsTest is Test {
    /// @notice Event emitted when a secret is revealed.
    event SecretRevealed(bytes32 indexed secretHash, bytes secret);

    /// @notice An instance of the CheckSecrets contract.
    CheckSecrets c;

    /// @notice A secret that must exist.
    bytes secretMustExist = bytes(string("secretMustExist"));

    /// @notice A secret that must not exist.
    bytes secretMustNotExist = bytes(string("secretMustNotExist"));

    /// @notice A delay period for the check.
    uint256 delay = 100;

    /// @notice Deploy the `CheckSecrets` contract.
    function setUp() external {
        c = new CheckSecrets();
    }

    /// @notice Test that the `name` function returns the correct value.
    function test_name_succeeds() external view {
        assertEq(c.name(), "CheckSecrets");
    }

    /// @notice Test that basic secret revealing works.
    function test_reveal_succeeds() external {
        // Simple reveal and check assertions.
        vm.expectEmit(address(c));
        emit SecretRevealed(keccak256(secretMustExist), secretMustExist);
        c.reveal(secretMustExist);
        assertEq(c.revealedSecrets(keccak256(secretMustExist)), block.timestamp);
    }

    /// @notice Test that revealing the same secret twice does not work.
    function test_reveal_twice_fails() external {
        // Reveal the secret once.
        uint256 ts = block.timestamp;
        c.reveal(secretMustExist);
        assertEq(c.revealedSecrets(keccak256(secretMustExist)), ts);

        // Forward time and reveal again, should fail, same original timestamp.
        vm.warp(ts + 1);
        vm.expectRevert("CheckSecrets: secret already revealed");
        c.reveal(secretMustExist);
        assertEq(c.revealedSecrets(keccak256(secretMustExist)), ts);
    }

    /// @notice Test that the check function returns true when the first secret is revealed but the
    ///         second secret is still hidden and the delay period has elapsed when the delay
    ///         period is non-zero. Here we warp to exactly the delay period.
    function test_check_secretRevealedWithDelayEq_succeeds() external {
        CheckSecrets.Params memory p = CheckSecrets.Params({
            delay: delay,
            secretHashMustExist: keccak256(secretMustExist),
            secretHashMustNotExist: keccak256(secretMustNotExist)
        });

        // Reveal the secret that must exist.
        c.reveal(secretMustExist);

        // Forward time to the delay period.
        vm.warp(block.timestamp + delay);

        // Beyond the delay, secret revealed, check should succeed.
        assertEq(c.check(abi.encode(p)), true);
    }

    /// @notice Test that the check function returns true when the first secret is revealed but the
    ///         second secret is still hidden and the delay period has elapsed when the delay
    ///         period is non-zero. Here we warp to after the delay period.
    function test_check_secretRevealedWithDelayGt_succeeds() external {
        CheckSecrets.Params memory p = CheckSecrets.Params({
            delay: delay,
            secretHashMustExist: keccak256(secretMustExist),
            secretHashMustNotExist: keccak256(secretMustNotExist)
        });

        // Reveal the secret that must exist.
        c.reveal(secretMustExist);

        // Forward time to after the delay period.
        vm.warp(block.timestamp + delay + 1);

        // Beyond the delay, secret revealed, check should succeed.
        assertEq(c.check(abi.encode(p)), true);
    }

    /// @notice Test that the check function returns true when the first secret is revealed but the
    ///         second secret is still hidden and the delay period is zero, meaning the reveal can
    ///         happen in the same block as the execution.
    function test_check_secretRevealedZeroDelay_succeeds() external {
        CheckSecrets.Params memory p = CheckSecrets.Params({
            delay: 0,
            secretHashMustExist: keccak256(secretMustExist),
            secretHashMustNotExist: keccak256(secretMustNotExist)
        });

        // Reveal the secret that must exist.
        c.reveal(secretMustExist);

        // Note we don't need to forward time here.
        // Secret revealed, no delay, check should succeed.
        assertEq(c.check(abi.encode(p)), true);
    }

    /// @notice Test that the check function returns false when the first secret is revealed but
    ///         the delay period has not yet elapsed.
    function test_check_secretRevealedBeforeDelay_fails() external {
        CheckSecrets.Params memory p = CheckSecrets.Params({
            delay: delay,
            secretHashMustExist: keccak256(secretMustExist),
            secretHashMustNotExist: keccak256(secretMustNotExist)
        });

        // Reveal the secret that must exist.
        c.reveal(secretMustExist);

        // Forward time to before the delay period.
        vm.warp(block.timestamp + delay - 1);

        // Not beyond the delay, check should fail.
        assertEq(c.check(abi.encode(p)), false);
    }

    /// @notice Test that the check function returns false when the first secret is not revealed.
    function test_check_secretNotRevealed_fails() external {
        CheckSecrets.Params memory p = CheckSecrets.Params({
            delay: delay,
            secretHashMustExist: keccak256(secretMustExist),
            secretHashMustNotExist: keccak256(secretMustNotExist)
        });

        // Forward beyond the delay period.
        vm.warp(block.timestamp + delay + 1);

        // Secret not revealed, check should fail.
        assertEq(c.check(abi.encode(p)), false);
    }

    /// @notice Test that the check function returns false when the second secret is revealed.
    function test_check_secondSecretRevealed_fails() external {
        CheckSecrets.Params memory p = CheckSecrets.Params({
            delay: delay,
            secretHashMustExist: keccak256(secretMustExist),
            secretHashMustNotExist: keccak256(secretMustNotExist)
        });

        // Reveal the secret that must not exist.
        c.reveal(secretMustNotExist);

        // Forward beyond the delay period.
        vm.warp(block.timestamp + delay + 1);

        // Both secrets revealed, check should fail.
        assertEq(c.check(abi.encode(p)), false);
    }

    /// @notice Test that the check function returns false when the second secret is revealed even
    ///         though the first secret is also revealed.
    function test_check_firstAndSecondSecretRevealed_fails() external {
        CheckSecrets.Params memory p = CheckSecrets.Params({
            delay: delay,
            secretHashMustExist: keccak256(secretMustExist),
            secretHashMustNotExist: keccak256(secretMustNotExist)
        });

        // Reveal the secret that must exist.
        c.reveal(secretMustExist);

        // Reveal the secret that must not exist.
        c.reveal(secretMustNotExist);

        // Forward beyond the delay period.
        vm.warp(block.timestamp + delay + 1);

        // Both secrets revealed, check should fail.
        assertEq(c.check(abi.encode(p)), false);
    }
}
