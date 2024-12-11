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

// If no predeploy natspec, disableInitializers can or cannot be called in constructor
contract SemgrepTest__sol_safety_use_disable_initializer {
    // ok: sol-safety-use-disable-initializer
    constructor() {
        // ...
        _disableInitializers();
        // ...
    }

    // ok: sol-safety-use-disable-initializer
    constructor() {
        // ...
    }
}

// if no predeploy natspec, disableInitializers must be called in constructor
/// @custom:proxied true
contract SemgrepTest__sol_safety_use_disable_initializer {
    // ok: sol-safety-use-disable-initializer
    constructor() {
        // ...
        _disableInitializers();
        // ...
    }

    // ruleid: sol-safety-use-disable-initializer
    constructor() {
        // ...
    }
}

/// NOTE: the predeploy natspec below is valid for all contracts after this one
/// @custom:predeploy
// if predeploy natspec, disableInitializers may or may not be called in constructor
contract SemgrepTest__sol_safety_use_disable_initializer {
    // ok: sol-safety-use-disable-initializer
    constructor() {
        // ...
    }

    // ok: sol-safety-use-disable-initializer
    constructor() {
        // ...
        _disableInitializers();
        // ...
    }
}
