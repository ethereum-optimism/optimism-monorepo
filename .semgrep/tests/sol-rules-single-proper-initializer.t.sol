// Semgrep tests for Solidity rules are defined in this file.
// Semgrep tests do not need to be valid Solidity code but should be syntactically correct so that
// Semgrep can parse them. You don't need to be able to *run* the code here but it should look like
// the code that you expect to catch with the rule.
//
// Semgrep testing 101
// Use comments like "ruleid: <rule-id>" to assert that the rule catches the code.
// Use comments like "ok: <rule-id>" to assert that the rule does not catch the code.

/// NOTE: The order here is important, because the proxied natspec identifier is used below, it is seen as present for
/// other tests after it's declaration and so we test for contracts without this natspec first

// If no proxied natspec, initialize functions can have no initializer modifier and be public or external
contract SemgrepTest__sol_safety_single_proper_initializer {
    // ok: sol-safety-single-proper-initializer
    function initialize() external {
        // ...
    }

    // ok: sol-safety-single-proper-initializer
    function initialize() public {
        // ...
    }
}

/// NOTE: the proxied natspec below is valid for all contracts after this one
/// @custom:proxied true
contract SemgrepTest__sol_safety_single_proper_initializer {
    // ok: sol-safety-single-proper-initializer
    function initialize() external initializer {
        // ...
    }

    // ruleid: sol-safety-single-proper-initializer
    function initialize() external {
        // ...
    }

    // ruleid: sol-safety-single-proper-initializer
    function initialize() public initializer {
        // ...
    }

    // ruleid: sol-safety-single-proper-initializer
    function initialize() public {
        // ...
    }
}
