GITCOMMIT ?= $(shell git rev-parse HEAD)
GITDATE ?= $(shell git show -s --format='%ct')
# Find the github tag that points to this commit. If none are found, set the version string to "untagged"
# Prioritizes release tag, if one exists, over tags suffixed with "-rc"
VERSION ?= $(shell tags=$$(git tag --points-at $(GITCOMMIT) | grep '^op-node/' | sed 's/op-node\///' | sort -V); \
             preferred_tag=$$(echo "$$tags" | grep -v -- '-rc' | tail -n 1); \
             if [ -z "$$preferred_tag" ]; then \
                 if [ -z "$$tags" ]; then \
                     echo "untagged"; \
                 else \
                     echo "$$tags" | tail -n 1; \
                 fi \
             else \
                 echo $$preferred_tag; \
             fi)

LDFLAGSSTRING +=-X main.GitCommit=$(GITCOMMIT)
LDFLAGSSTRING +=-X main.GitDate=$(GITDATE)
LDFLAGSSTRING +=-X github.com/ethereum-optimism/optimism/op-node/version.Version=$(VERSION)
LDFLAGSSTRING +=-X github.com/ethereum-optimism/optimism/op-node/version.Meta=$(VERSION_META)
LDFLAGS := -ldflags "$(LDFLAGSSTRING)"

# Use the old Apple linker to workaround broken xcode - https://github.com/golang/go/issues/65169
ifeq ($(shell uname),Darwin)
	FUZZLDFLAGS := -ldflags=-extldflags=-Wl,-ld_classic
endif

op-node:
	env GO111MODULE=on GOOS=$(TARGETOS) GOARCH=$(TARGETARCH) CGO_ENABLED=0 go build -v $(LDFLAGS) -o ./bin/op-node ./cmd/main.go

clean:
	rm bin/op-node

test:
	go test -v ./...

fuzz:
	printf "%s\n" \
		"go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz FuzzL1InfoBedrockRoundTrip ./rollup/derive" \
		"go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz FuzzL1InfoEcotoneRoundTrip ./rollup/derive" \
		"go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz FuzzL1InfoAgainstContract ./rollup/derive" \
		"go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz FuzzUnmarshallLogEvent ./rollup/derive" \
		"go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz FuzzParseFrames ./rollup/derive" \
		"go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz FuzzFrameUnmarshalBinary ./rollup/derive" \
		"go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz FuzzBatchRoundTrip ./rollup/derive" \
		"go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz FuzzDeriveDepositsRoundTrip ./rollup/derive" \
		"go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz FuzzDeriveDepositsBadVersion ./rollup/derive" \
		"go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz FuzzParseL1InfoDepositTxDataValid ./rollup/derive" \
		"go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz FuzzParseL1InfoDepositTxDataBadLength ./rollup/derive" \
		"go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz FuzzRejectCreateBlockBadTimestamp ./rollup/driver" \
		"go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz FuzzDecodeDepositTxDataToL1Info ./rollup/driver" \
	| parallel -j 8 {}

generate-mocks:
	go generate ./...

readme:
	doctoc README.md

.PHONY: \
	op-node \
	clean \
	test \
	fuzz \
	readme
