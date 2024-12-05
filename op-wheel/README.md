# `op-wheel`

Issues: [monorepo](https://github.com/ethereum-optimism/optimism/issues?q=is%3Aissue%20state%3Aopen%20label%3AA-op-wheel)

Pull requests: [monorepo](https://github.com/ethereum-optimism/optimism/pulls?q=is%3Aopen+is%3Apr+label%3AA-op-wheel)

`op-wheel` is a CLI tool to direct the engine one way or the other with DB cheats and Engine API routines.

It was named the "wheel" because of two reasons:
- Figuratively, it allows to steer the stack, an interface for a *driver* (like the op-node sub-component) to control the execution *engine* (e.g. op-geth).
- Idiomatically, like the Unix wheel-bit and its slang origins: empower a user to execute restricted commands, or more generally just someone with great power or influence.

## Quickstart

### Cheat utils

Cheating commands to modify a Geth database without corresponding in-protocol change.

The `cheat` sub-command has sub-commands for interacting with the DB, making patches, and dumping debug data.

Note that the validity of state-changes, as applied through patches,
does not get checked until the block is re-processed.
This can be used ot trick the node into things like hypothetical
test-states or shadow-forks without diverging the block-hashes.

To run:
```bash
go run ./op-wheel/cmd cheat --help
```

### Engine utils

Engine API commands to build/reorg/rewind/finalize/copy blocks.

Each sub-command dials the engine API endpoint (with provided JWT secret) and then runs the action.

To run:
```bash
go run ./op-wheel/cmd engine --help
```

## Usage

### Build from source

```bash
# from op-wheel dir:
make op-wheel
./bin/op-wheel --help
```

### Run from source

```bash
# from op-wheel dir:
go run ./cmd --help
```

### Build docker image

See `op-wheel` docker-bake target.

## Product

`op-wheel` is a tool for expert-users to perform advanced data recoveries, tests and overrides.
This tool optimizes for reusability of these expert actions, to make them less error-prone.

This is not part of a standard release / process, as this tool is not used commonly,
and the end-user is expected to be familiar with building from source.

Actions that are common enough to be used at least once by the average end-user should
be part of the op-node or other standard op-stack release.

## Design principles

Design for an expert-user: this tool aims to provide full control over critical op-stack data
such as the engine-API and database itself, without hiding important information.

However, even as expert-user, wrong assumptions can be made.
Defaults should aim to reduce errors, and leave the stack in a safe state to recover from.

## Failure modes

This tool is not used in the happy-path, but can be critical during expert-recovery of advanced failure modes.
E.g. database recovery after Geth database corruption, or manual forkchoice overrides.
Most importantly, each CLI command used for recovery aims to be verbose,
and avoids leaving an inconsistent state after failed or interrupted recovery.

## Testing

This is a test-utility more than a production tool, and thus does currently not have test-coverage of its own.
However, when it is used as tool during (dev/test) chain or node issues, usage does inform fixes/improvements.
