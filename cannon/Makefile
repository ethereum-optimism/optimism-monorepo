GITCOMMIT ?= $(shell git rev-parse HEAD)
GITDATE ?= $(shell git show -s --format='%ct')
VERSION ?= v0.0.0

LDFLAGSSTRING +=-X main.GitCommit=$(GITCOMMIT)
LDFLAGSSTRING +=-X main.GitDate=$(GITDATE)
LDFLAGSSTRING +=-X github.com/ethereum-optimism/optimism/cannon/multicannon/version.Version=$(VERSION)
LDFLAGSSTRING +=-X github.com/ethereum-optimism/optimism/cannon/multicannon/version.Meta=$(VERSION_META)
LDFLAGS := -ldflags "$(LDFLAGSSTRING)"

# Use the old Apple linker to workaround broken xcode - https://github.com/golang/go/issues/65169
ifeq ($(shell uname),Darwin)
	FUZZLDFLAGS := -ldflags=-extldflags=-Wl,-ld_classic
endif

.DEFAULT_GOAL := cannon

cannon32-impl:
	env GO111MODULE=on GOOS=$(TARGETOS) GOARCH=$(TARGETARCH) go build --tags=cannon32 -v $(LDFLAGS) -o ./bin/cannon32-impl .

cannon64-impl:
	env GO111MODULE=on GOOS=$(TARGETOS) GOARCH=$(TARGETARCH) go build --tags=cannon64 -v $(LDFLAGS) -o ./bin/cannon64-impl .

# Note: This target is used by ./scripts/build-legacy-cannons.sh
# It should build the individual versions of cannons and copy them into place in hte multicannon/embeds directory
# Ideally, preserve backwards compatibility with this behaviour but if it needs to change, build-legacy-cannons.sh will
# need to be updated to account for different behaviours in different versions.
cannon-embeds: cannon32-impl cannon64-impl
	# singlethreaded-v2
	@cp bin/cannon32-impl ./multicannon/embeds/cannon-2
	# multithreaded
	@cp bin/cannon32-impl ./multicannon/embeds/cannon-1
	# 64-bit multithreaded
	@cp bin/cannon64-impl ./multicannon/embeds/cannon-3

cannon: cannon-embeds
	env GO111MODULE=on GOOS=$(TARGETOS) GOARCH=$(TARGETARCH) go build -v $(LDFLAGS) -o ./bin/cannon ./multicannon/

clean:
	rm -rf bin multicannon/embeds/cannon*

elf:
	make -C ./testdata/example elf

sanitize-program:
	@if ! { mips-linux-gnu-objdump -d -j .text $$GUEST_PROGRAM | awk '{print $3}' | grep -Ew -m1 '(bltzal)'; }; then \
		echo "guest program is sanitized for unsupported instructions"; \
	else \
		echo "found unsupported instructions in the guest program"; \
		exit 1; \
	fi

contract:
	cd ../packages/contracts-bedrock && forge build

test: elf contract
	go test -v ./...

diff-%-cannon: cannon elf
	$$OTHER_CANNON load-elf --type $* --path ./testdata/example/bin/hello.elf --out ./bin/prestate-other.bin.gz --meta ""
	./bin/cannon   load-elf --type $* --path ./testdata/example/bin/hello.elf --out ./bin/prestate.bin.gz --meta ""
	@cmp ./bin/prestate-other.bin.gz ./bin/prestate.bin.gz
	@if [ $$? -eq 0 ]; then \
		echo "Generated identical prestates"; \
	else \
		echo "Generated different prestates"; \
		exit 1; \
	fi
	$$OTHER_CANNON run --proof-at '=0' --stop-at '=100000000' --input=./bin/prestate.bin.gz  --output ./bin/out-other.bin.gz --meta ""
	./bin/cannon   run --proof-at '=0' --stop-at '=100000000' --input=./bin/prestate.bin.gz  --output ./bin/out.bin.gz --meta ""
	@cmp ./bin/out-other.bin.gz ./bin/out.bin.gz
	@if [ $$? -eq 0 ]; then \
		echo "Generated identical states"; \
	else \
		echo "Generated different prestates"; \
		exit 1; \
	fi

cannon-stf-verify:
	@docker build --progress plain -f Dockerfile.diff ../

fuzz:
  # Common vm tests
	go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz=FuzzStateSyscallBrk ./mipsevm/tests
	go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz=FuzzStateSyscallMmap ./mipsevm/tests
	go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz=FuzzStateSyscallExitGroup ./mipsevm/tests
	go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz=FuzzStateSyscallFcntl ./mipsevm/tests
	go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz=FuzzStateHintRead ./mipsevm/tests
	go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 20s -fuzz=FuzzStatePreimageRead ./mipsevm/tests
	go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz=FuzzStateHintWrite ./mipsevm/tests
	go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 20s -fuzz=FuzzStatePreimageWrite ./mipsevm/tests
	# Single-threaded tests
	go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz=FuzzStateSyscallCloneST ./mipsevm/tests
	# Multi-threaded tests
	go test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz=FuzzStateSyscallCloneMT ./mipsevm/tests

.PHONY: \
	cannon32-impl \
	cannon64-impl \
	cannon-embeds \
	cannon \
	clean \
	test \
	lint \
	fuzz \
	diff-%-cannon \
	cannon-stf-verify
