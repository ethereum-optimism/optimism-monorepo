# provide JUSTFLAGS for just-backed targets
include ./justfiles/flags.mk

BEDROCK_TAGS_REMOTE?=origin
OP_STACK_GO_BUILDER?=us-docker.pkg.dev/oplabs-tools-artifacts/images/op-stack-go:latest

# Requires at least Python v3.9; specify a minor version below if needed
PYTHON?=python3

help: ## Prints this help message
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: build-go build-contracts ## Builds Go components and contracts-bedrock
.PHONY: build

build-go: submodules op-node op-proposer op-batcher ## Builds op-node, op-proposer and op-batcher
.PHONY: build-go

build-contracts:
	(cd packages/contracts-bedrock && just build)
.PHONY: build-contracts

lint-go: ## Lints Go code with specific linters
	golangci-lint run -E goimports,sqlclosecheck,bodyclose,asciicheck,misspell,errorlint --timeout 5m -e "errors.As" -e "errors.Is" ./...
.PHONY: lint-go

lint-go-fix: ## Lints Go code with specific linters and fixes reported issues
	golangci-lint run -E goimports,sqlclosecheck,bodyclose,asciicheck,misspell,errorlint --timeout 5m -e "errors.As" -e "errors.Is" ./... --fix
.PHONY: lint-go-fix

golang-docker: ## Builds Docker images for Go components using buildx
	# We don't use a buildx builder here, and just load directly into regular docker, for convenience.
	GIT_COMMIT=$$(git rev-parse HEAD) \
	GIT_DATE=$$(git show -s --format='%ct') \
	IMAGE_TAGS=$$(git rev-parse HEAD),latest \
	docker buildx bake \
			--progress plain \
			--load \
			-f docker-bake.hcl \
			op-node op-batcher op-proposer op-challenger op-dispute-mon op-supervisor
.PHONY: golang-docker

docker-builder-clean: ## Removes the Docker buildx builder
	docker buildx rm buildx-build
.PHONY: docker-builder-clean

docker-builder: ## Creates a Docker buildx builder
	docker buildx create \
		--driver=docker-container --name=buildx-build --bootstrap --use
.PHONY: docker-builder

# add --print to dry-run
cross-op-node: ## Builds cross-platform Docker image for op-node
	# We don't use a buildx builder here, and just load directly into regular docker, for convenience.
	GIT_COMMIT=$$(git rev-parse HEAD) \
	GIT_DATE=$$(git show -s --format='%ct') \
	IMAGE_TAGS=$$(git rev-parse HEAD),latest \
	PLATFORMS="linux/arm64" \
	GIT_VERSION=$(shell tags=$$(git tag --points-at $(GITCOMMIT) | grep '^op-node/' | sed 's/op-node\///' | sort -V); \
             preferred_tag=$$(echo "$$tags" | grep -v -- '-rc' | tail -n 1); \
             if [ -z "$$preferred_tag" ]; then \
                 if [ -z "$$tags" ]; then \
                     echo "untagged"; \
                 else \
                     echo "$$tags" | tail -n 1; \
                 fi \
             else \
                 echo $$preferred_tag; \
             fi) \
	docker buildx bake \
			--progress plain \
			--builder=buildx-build \
			--load \
			--no-cache \
			-f docker-bake.hcl \
			op-node
.PHONY: cross-op-node

contracts-bedrock-docker: ## Builds Docker image for Bedrock contracts
	IMAGE_TAGS=$$(git rev-parse HEAD),latest \
	docker buildx bake \
			--progress plain \
			--load \
			-f docker-bake.hcl \
		  contracts-bedrock
.PHONY: contracts-bedrock-docker

submodules: ## Updates git submodules
	git submodule update --init --recursive
.PHONY: submodules


op-node: ## Builds op-node binary
	just $(JUSTFLAGS) ./op-node/op-node
.PHONY: op-node

generate-mocks-op-node: ## Generates mocks for op-node
	make -C ./op-node generate-mocks
.PHONY: generate-mocks-op-node

generate-mocks-op-service: ## Generates mocks for op-service
	make -C ./op-service generate-mocks
.PHONY: generate-mocks-op-service

op-batcher: ## Builds op-batcher binary
	just $(JUSTFLAGS) ./op-batcher/op-batcher
.PHONY: op-batcher

op-proposer: ## Builds op-proposer binary
	just $(JUSTFLAGS) ./op-proposer/op-proposer
.PHONY: op-proposer

op-challenger: ## Builds op-challenger binary
	make -C ./op-challenger op-challenger
.PHONY: op-challenger

op-dispute-mon: ## Builds op-dispute-mon binary
	make -C ./op-dispute-mon op-dispute-mon
.PHONY: op-dispute-mon

op-program: ## Builds op-program binary
	make -C ./op-program op-program
.PHONY: op-program

cannon:  ## Builds cannon binary
	make -C ./cannon cannon
.PHONY: cannon

reproducible-prestate:   ## Builds reproducible-prestate binary
	make -C ./op-program reproducible-prestate
.PHONY: reproducible-prestate

# Include any files required for the devnet to build and run.
DEVNET_CANNON_PRESTATE_FILES := op-program/bin/prestate-proof.json op-program/bin/prestate.bin.gz op-program/bin/prestate-proof-mt64.json op-program/bin/prestate-mt64.bin.gz op-program/bin/prestate-interop.bin.gz


$(DEVNET_CANNON_PRESTATE_FILES):
	make cannon-prestate
	make cannon-prestate-mt64
	make cannon-prestate-interop

cannon-prestates: cannon-prestate cannon-prestate-mt64 cannon-prestate-interop
.PHONY: cannon-prestates

cannon-prestate: op-program cannon ## Generates prestate using cannon and op-program
	./cannon/bin/cannon load-elf --type singlethreaded-2 --path op-program/bin/op-program-client.elf --out op-program/bin/prestate.bin.gz --meta op-program/bin/meta.json
	./cannon/bin/cannon run --proof-at '=0'  --stop-at '=1' --input op-program/bin/prestate.bin.gz --meta op-program/bin/meta.json --proof-fmt 'op-program/bin/%d.json' --output ""
	mv op-program/bin/0.json op-program/bin/prestate-proof.json
.PHONY: cannon-prestate

cannon-prestate-mt64: op-program cannon ## Generates prestate using cannon and op-program in the latest 64-bit multithreaded cannon format
	./cannon/bin/cannon load-elf --type multithreaded64-3 --path op-program/bin/op-program-client64.elf --out op-program/bin/prestate-mt64.bin.gz --meta op-program/bin/meta-mt64.json
	./cannon/bin/cannon run --proof-at '=0' --stop-at '=1' --input op-program/bin/prestate-mt64.bin.gz --meta op-program/bin/meta-mt64.json --proof-fmt 'op-program/bin/%d-mt64.json' --output ""
	mv op-program/bin/0-mt64.json op-program/bin/prestate-proof-mt64.json
.PHONY: cannon-prestate-mt64

cannon-prestate-interop: op-program cannon ## Generates interop prestate using cannon and op-program in the latest 64-bit multithreaded cannon format
	./cannon/bin/cannon load-elf --type multithreaded64-3 --path op-program/bin/op-program-client-interop.elf --out op-program/bin/prestate-interop.bin.gz --meta op-program/bin/meta-interop.json
	./cannon/bin/cannon run --proof-at '=0' --stop-at '=1' --input op-program/bin/prestate-interop.bin.gz --meta op-program/bin/meta-interop.json --proof-fmt 'op-program/bin/%d-interop.json' --output ""
	mv op-program/bin/0-interop.json op-program/bin/prestate-proof-interop.json
.PHONY: cannon-prestate-interop

mod-tidy: ## Cleans up unused dependencies in Go modules
	# Below GOPRIVATE line allows mod-tidy to be run immediately after
	# releasing new versions. This bypasses the Go modules proxy, which
	# can take a while to index new versions.
	#
	# See https://proxy.golang.org/ for more info.
	export GOPRIVATE="github.com/ethereum-optimism" && go mod tidy
.PHONY: mod-tidy

clean: ## Removes all generated files under bin/
	rm -rf ./bin
	cd packages/contracts-bedrock/ && forge clean
.PHONY: clean

nuke: clean ## Completely clean the project directory
	git clean -Xdf
.PHONY: nuke

test-unit: ## Runs unit tests for all components
	make -C ./op-node test
	make -C ./op-proposer test
	make -C ./op-batcher test
	make -C ./op-e2e test
	(cd packages/contracts-bedrock && just test)
.PHONY: test-unit

# Remove the baseline-commit to generate a base reading & show all issues
semgrep: ## Runs Semgrep checks
	$(eval DEV_REF := $(shell git rev-parse develop))
	SEMGREP_REPO_NAME=ethereum-optimism/optimism semgrep ci --baseline-commit=$(DEV_REF)
.PHONY: semgrep

update-op-geth: ## Updates the Geth version used in the project
	./ops/scripts/update-op-geth.py
.PHONY: update-op-geth
