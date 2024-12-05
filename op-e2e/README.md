# `op-e2e`

Issues: [monorepo](https://github.com/ethereum-optimism/optimism/issues?q=is%3Aissue%20state%3Aopen%20label%3AA-op-e2e)

Pull requests: [monorepo](https://github.com/ethereum-optimism/optimism/pulls?q=is%3Aopen+is%3Apr+label%3AA-op-e2e)

Design docs:
- [test infra draft design-doc]: active discussion of end-to-end testing approach

[test infra draft design-doc](https://github.com/ethereum-optimism/design-docs/pull/165)

`op-e2e` is a collection of Go integration tests.
It is named `e2e` after end-to-end testing,
for those tests where we integration-test the full system, rather than only specific services.


## Quickstart

```bash
make test-actions
make test-ws
```

## Overview

`op-e2e` can be categorized as following:
- `op-e2e/actions/`: imperative test style, more DSL-like, with a focus on the state-transition parts of services.
  Parallel processing is actively avoided, and a mock clock is used.
  - `op-e2e/actions/*`: sub-packages categorize specific domains to test.
  - `op-e2e/actions/interop`: notable sub-package, where multiple L2s are attached together,
    for integration-testing across multiple L2 chains.
  - `op-e2e/actions/proofs`: notable sub-package, where proof-related state-transition testing is implemented,
    with experimental support to cover alternative proof implementations.
- `op-e2e/system`: integration tests with a L1 miner and a L2 with sequencer, verifier, batcher and proposer.
  These tests do run each service almost fully, including parallel background jobs and real system clock.
  These tests focus less on the onchain state-transition aspects, and more on the offchain integration aspects.
  - `op-e2e/faultproofs`: system tests with fault-proofs stack attached
  - `op-e2e/interop`: system tests with a distinct Interop "SuperSystem", to run multiple L2 chains.
- `op-e2e/opgeth`: integration tests between test-mocks and op-geth execution-engine.
  - also includes upgrade-tests to ensure testing of op-stack Go components around a network upgrade.

### `action`-tests

Action tests are set up in a compositional way:
each service is instantiated as actor, and tests can choose to run just the relevant set of actors.
E.g. a test about data-availability can instantiate the batcher, but omit the proposer.

One action, across all services, runs at a time.
No live background processing or system clock affects the actors:
this enables individual actions to be deterministic and reproducible.

With this synchronous processing, action-test can reliably navigate towards
these otherwise hard-to-reach edge-cases, and ensure the state-transition of service,
and the interactions between this state, are covered.

Action-tests do not cover background processes or peripherals.
E.g. P2P, CLI usage, and dynamic block building are not covered.

### `system`-tests

System tests are more complete than `action` tests, but also require a live system.
This trade-off enables coverage of most of each Go service,
at the cost of making navigation to cover the known edge-cases less reliable and reproducible.
This test-type is thus used primarily for testing of the offchain service aspects.

By running a more full system, test-runners also run into resource-limits more quickly.
This may result in lag or even stalled services.
Improvements, as described in the [test infra draft design-doc],
are in active development, to make test execution more reliable.

### `op-e2e/opgeth`

Integration-testing with op-geth, to cover engine behavior, without setting up a full test environment.
These tests are limited in scope, and may be changed at a later stage, to support alternative EL implementations.

## Product

### Optimization target

Historically `op-e2e` has been optimized for test-coverage of the Go OP-Stack.
This is changing with the advance of alternative OP-Stack client implementations.

New test framework improvements should optimize for multi-client testing.

### Vision

Generally, design-discussion and feedback from active test users converges on:
- a need to share test-resources, to host more tests while reducing overhead.
- a need for a DSL, to better express common test constructs.
- less involved test pre-requisites: the environment should be light and simple, welcoming new contributors.
  E.g. no undocumented one-off makefile prerequisites.

## Design principles

- Interfaces first. We should not hardcode test-utilities against any specific client implementation,
  this makes a test less parameterizable and less cross-client portable.
- Abstract setup to make it the default to reduce resource usage.
  E.g. RPC transports can run in-process, and avoid websocket or HTTP costs,
  and ideally the test-writer does not have to think about the difference.
- Avoid one-off test chain-configurations. Tests with more realistic parameters are more comparable to production,
  and easier consolidated onto shared testing resources.
- Write helpers and DSL utilities, avoid re-implementing common testing steps.
  The better the test environment, the more inviting it is for someone new to help improve test coverage.
- Use the right test-type. Do not spawn a full system for something of very limited scope,
  e.g. when it fits better in a unit-test.
