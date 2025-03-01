# op-test-sequencer

This is a test service for block sequencing.
This service is in active development.

## Usage

### Build from source

```bash
# from op-test-sequencer dir:
just op-test-sequencer
./bin/op-test-sequencer --help
```

### Run from source

```bash
# from op-test-sequencer dir:
go run ./cmd --help
```

### Build docker image

Not available yet.

## Overview

This service is in active development.

See [design doc](https://github.com/ethereum-optimism/design-docs/blob/main/protocol/test-sequencing.md)
for design considerations.

### RPC

On the configured RPC address/port multiple HTTP routes are served, each serving RPCs.
Every RPC is authenticated with a JWT-secret.
For every route, both HTTP and Websocket RPC connections are supported.

#### Main RPC

The main RPC is served on the root path `/` of the configured RPC host/port.

##### `admin`

Work in progress.

##### `build`

Types:
- `BuilderID`: string, identifies a builder by its configured name
- `BuildJobID`: string, identifies a build job
- `BuildOpts`: `{parent: hash, l1Origin: hash,optional}` (work in progress, will be extended)
- `Block`: block, a JSON object, as defined by the builder

Methods:
- `build_open(id: BuilderID, opts: BuildOpts) -> BuildJobID`
- `build_cancel(jobID: BuildJobID)`
- `build_seal(jobID: BuildJobID) -> Block`

#### Sequencer RPC routes

`/sequencers/{sequencerID}` serves an RPC (Both HTTP and Websocket)

##### `sequencer`

Actively changing. Methods to run through each part of the sequencing flow.
See [`sequencer/frontend/sequencer.go`](./sequencer/frontend/sequencer.go).

