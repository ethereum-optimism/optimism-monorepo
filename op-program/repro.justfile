GIT_COMMIT := ""
GIT_DATE := ""

CANNON_VERSION := "v0.0.0"
OP_PROGRAM_VERSION := "v0.0.0"

GOOS := ""
GOARCH := ""

# Build the cannon binary
cannon:
    #!/bin/bash
    # in devnet scenario, the cannon binary is already built.
    [ -x /app/cannon/bin/cannon ] && exit 0
    cd ../cannon
    make cannon \
        GOOS={{GOOS}} \
        GOARCH={{GOARCH}} \
        GITCOMMIT={{GIT_COMMIT}} \
        GITDATE={{GIT_DATE}} \
        VERSION={{CANNON_VERSION}}

# Build the op-program-client elf binaries
op-program-client-mips:
    #!/bin/bash
    cd ../op-program
    make op-program-client-mips \
        GOOS=linux \
        GOARCH=mips \
        GOMIPS=softfloat \
        GITCOMMIT={{GIT_COMMIT}} \
        GITDATE={{GIT_DATE}} \
        VERSION={{OP_PROGRAM_VERSION}}

# Run the op-program-client elf binary directly through cannon's load-elf subcommand.
client TYPE CLIENT_SUFFIX PRESTATE_SUFFIX: cannon op-program-client-mips
    #!/bin/bash
    /app/cannon/bin/cannon load-elf \
        --type {{TYPE}} \
        --path /app/op-program/bin/op-program-client{{CLIENT_SUFFIX}}.elf \
        --out /app/op-program/bin/prestate{{PRESTATE_SUFFIX}}.bin.gz \
        --meta "/app/op-program/bin/meta{{PRESTATE_SUFFIX}}.json"

# Generate the prestate proof containing the absolute pre-state hash.
prestate TYPE CLIENT_SUFFIX PRESTATE_SUFFIX: (client TYPE CLIENT_SUFFIX PRESTATE_SUFFIX)
    #!/bin/bash
    /app/cannon/bin/cannon run \
        --proof-at '=0' \
        --stop-at '=1' \
        --input /app/op-program/bin/prestate{{PRESTATE_SUFFIX}}.bin.gz \
        --meta "" \
        --proof-fmt '/app/op-program/bin/%d{{PRESTATE_SUFFIX}}.json' \
        --output ""
    mv /app/op-program/bin/0{{PRESTATE_SUFFIX}}.json /app/op-program/bin/prestate-proof{{PRESTATE_SUFFIX}}.json

build-default: (prestate "singlethreaded-2" "" "")
build-mt64: (prestate "multithreaded64-3" "64" "-mt64")
build-interop: (prestate "multithreaded64-3" "-interop" "-interop")

build-all: build-default build-mt64 build-interop
