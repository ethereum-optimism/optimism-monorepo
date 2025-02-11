########################################################
#                        INSTALL                       #
########################################################

# Installs dependencies.
install:
  forge install

# Shows the status of the git submodules.
dep-status:
  git submodule status


########################################################
#                         BUILD                        #
########################################################

# Core forge build command.
forge-build *ARGS:
  forge build {{ARGS}}

# Developer build command (faster).
forge-build-dev *ARGS:
  FOUNDRY_PROFILE=lite forge build {{ARGS}}

# Builds source contracts only.
build-source:
  forge build --skip "/**/test/**" --skip "/**/scripts/**"

# Builds the contracts.
build *ARGS: lint-fix-no-fail
  just forge-build {{ARGS}}
  just interfaces-check-no-build

# Builds the contracts (developer mode).
build-dev *ARGS: lint-fix-no-fail
  just forge-build-dev {{ARGS}}
  just interfaces-check-no-build

# Builds the go-ffi tool for contract tests.
build-go-ffi-default:
  cd ./scripts/go-ffi && go build

# Builds the go-ffi tool for MIPS64 contract tests.
build-go-ffi-cannon64:
  cd ./scripts/go-ffi && go build -tags=cannon64 -o ./go-ffi-cannon64

build-go-ffi: build-go-ffi-default build-go-ffi-cannon64

# Cleans build artifacts and deployments.
clean:
  rm -rf ./artifacts ./forge-artifacts ./cache ./scripts/go-ffi/go-ffi ./deployments/hardhat/*


########################################################
#                         TEST                         #
########################################################

# Runs standard contract tests.
test *ARGS: build-go-ffi
  forge test {{ARGS}}

# Runs standard contract tests (developer mode).
test-dev *ARGS: build-go-ffi
  FOUNDRY_PROFILE=lite forge test {{ARGS}}

# Default block number for the forked upgrade path.
export pinnedBlockNumber := env_var_or_default('FORK_BLOCK_NUMBER', '21387830')
print-pinned-block-number:
  echo $pinnedBlockNumber

# Prepares the environment for upgrade path variant of contract tests and coverage.
# Env Vars:
# - ETH_RPC_URL must be set to a production (Sepolia or Mainnet) RPC URL.
# - FORK_BLOCK_NUMBER can be set in the env, or else will fallback to the default block number.
#   Reusing the default block number greatly speeds up the test execution time by caching the
#   rpc call responses in ~/.foundry/cache/rpc. The default block will need to be updated
#   when the L1 chain is upgraded.
# TODO(opcm upgrades): unskip the "NMC" tests which fail on the forked upgrade path with "UnexpectedRootClaim" errors.
prepare-upgrade-env *ARGS : build-go-ffi
  #!/bin/bash
  echo "Running upgrade tests at block $pinnedBlockNumber"
  export FORK_BLOCK_NUMBER=$pinnedBlockNumber
  export NO_MATCH_CONTRACTS="OptimismPortal2WithMockERC20_Test|OptimismPortal2_FinalizeWithdrawal_Test|AnchorStateRegistry_*|FaultDisputeGame_Test|PermissionedDisputeGame_Test|FaultDispute_1v1_Actors_Test|DelayedWETH_Hold_Test"
  export FORK_RPC_URL=$ETH_RPC_URL
  export FORK_TEST=true
  {{ARGS}} \
  --match-path "test/{L1,dispute,cannon}/**" \
  --no-match-contract "$NO_MATCH_CONTRACTS"

# Runs upgrade path variant of contract tests.
test-upgrade *ARGS:
  just prepare-upgrade-env "forge test {{ARGS}}"

test-upgrade-rerun *ARGS: build-go-ffi
  just test-upgrade {{ARGS}} --rerun -vvv

# Starts a local anvil node with a mainnet fork and sends it to the background
# Requires ETH_RPC_URL to be set to a production (Sepolia or Mainnet) RPC URL.
anvil-fork:
  anvil --fork-url $ETH_RPC_URL

# Use anvil-fork in a separate terminal before running this command.
# Helpful for debugging.
test-upgrade-against-anvil *ARGS: build-go-ffi
  #!/bin/bash
  echo "Running upgrade tests at block $pinnedBlockNumber"
  export FORK_BLOCK_NUMBER=$pinnedBlockNumber
  export NO_MATCH_CONTRACTS="OptimismPortal2WithMockERC20_Test|OptimismPortal2_FinalizeWithdrawal_Test|AnchorStateRegistry_*|FaultDisputeGame_Test|PermissionedDisputeGame_Test|FaultDispute_1v1_Actors_Test|DelayedWETH_Hold_Test"
  export FORK_RPC_URL=http://127.0.0.1:8545
  export FORK_TEST=true
  forge test {{ARGS}} \
  --match-path "test/{L1,dispute,cannon}/**" \
  --no-match-contract "$NO_MATCH_CONTRACTS"

# Runs standard contract tests with rerun flag.
test-rerun: build-go-ffi
  forge test --rerun -vvv

# Runs standard contract tests with rerun flag (developer mode).
test-dev-rerun: build-go-ffi
  FOUNDRY_PROFILE=lite forge test --rerun -vvv

# Run Kontrol tests and build all dependencies.
test-kontrol: build-go-ffi build kontrol-summary-full test-kontrol-no-build

# Run Kontrol tests without dependencies.
test-kontrol-no-build:
  ./test/kontrol/scripts/run-kontrol.sh script

# Runs contract coverage.
coverage: build-go-ffi
  forge coverage

# Runs contract coverage with lcov.
coverage-lcov *ARGS: build-go-ffi
  forge coverage {{ARGS}} --report lcov --report-file lcov.info

# Runs upgrade path variant of contract coverage tests.
coverage-upgrade *ARGS:
  just prepare-upgrade-env "forge coverage {{ARGS}}"

# Runs contract coverage with lcov.
coverage-lcov-upgrade *ARGS: build-go-ffi
  just coverage-upgrade {{ARGS}} --report lcov --report-file lcov-upgrade.info

# Runs coverage-lcov and coverage-lcov-upgrade and merges their output files info one file
coverage-lcov-all *ARGS:
  just coverage-lcov {{ARGS}} && \
  just coverage-lcov-upgrade --match-contract OPContractsManager_Upgrade_Test {{ARGS}} && \
  lcov -a lcov.info -a lcov-upgrade.info -o lcov-all.info

########################################################
#                        DEPLOY                        #
########################################################

# Generates the L2 genesis state.
genesis:
  forge script scripts/L2Genesis.s.sol:L2Genesis --sig 'runWithStateDump()'

# Deploys the contracts.
deploy:
  ./scripts/deploy/deploy.sh


########################################################
#                       SNAPSHOTS                      #
########################################################

# Generates default Kontrol summary.
kontrol-summary:
  ./test/kontrol/scripts/make-summary-deployment.sh

# Generates fault proofs Kontrol summary.
kontrol-summary-fp:
  KONTROL_FP_DEPLOYMENT=true ./test/kontrol/scripts/make-summary-deployment.sh

# Generates all Kontrol summaries (default and FP).
kontrol-summary-full: kontrol-summary kontrol-summary-fp

# Generates ABI snapshots for contracts.
snapshots-abi-storage-no-build:
  go run ./scripts/autogen/generate-snapshots .

# Generates ABI snapshots for contracts.
snapshots-abi-storage: build-source snapshots-abi-storage-no-build

# Updates the snapshots/semver-lock.json file without building contracts.
semver-lock-no-build:
  go run scripts/autogen/generate-semver-lock/main.go

# Updates the snapshots/semver-lock.json file.
semver-lock: build-source semver-lock-no-build

# Generates core snapshots without building contracts.
snapshots-no-build: snapshots-abi-storage-no-build semver-lock-no-build

# Builds contracts and then generates core snapshots.
snapshots: build-source snapshots-no-build


########################################################
#                        CHECKS                        #
########################################################

# Checks if the snapshots are up to date without building.
snapshots-check-no-build: snapshots-no-build

# Checks if the snapshots are up to date.
snapshots-check: build snapshots-check-no-build

# Checks interface correctness without building.
interfaces-check-no-build:
  go run ./scripts/checks/interfaces

# Checks that all interfaces are appropriately named and accurately reflect the corresponding
# contract that they're meant to represent. We run "clean" before building because leftover
# artifacts can cause the script to detect issues incorrectly.
interfaces-check: clean build interfaces-check-no-build

# Checks that the size of the contracts is within the limit.
size-check:
  forge build --sizes --skip "/**/test/**" --skip "/**/scripts/**"

# Checks that any contracts with a modified semver lock also have a modified semver version.
# Does not build contracts.
semver-diff-check-no-build:
  ./scripts/checks/check-semver-diff.sh

# Checks that any contracts with a modified semver lock also have a modified semver version.
semver-diff-check: build semver-diff-check-no-build

# Checks that the semgrep tests are valid.
semgrep-test-validity-check:
  forge fmt ../../.semgrep/tests/sol-rules.t.sol --check

# Checks that forge test names are correctly formatted. Does not build contracts.
lint-forge-tests-check-no-build:
  go run ./scripts/checks/test-names

# Checks that forge test names are correctly formatted.
lint-forge-tests-check: build lint-forge-tests-check-no-build

# Checks that contracts are properly linted.
lint-check:
  forge fmt --check

# Checks for unused imports in Solidity contracts. Does not build contracts.
unused-imports-check-no-build:
  go run ./scripts/checks/unused-imports

# Checks for unused imports in Solidity contracts.
unused-imports-check: build unused-imports-check-no-build

# Checks that the deploy configs are valid.
validate-deploy-configs:
  ./scripts/checks/check-deploy-configs.sh

# Checks that spacer variables are correctly inserted without building.
validate-spacers-no-build:
  go run ./scripts/checks/spacers

# Checks that spacer variables are correctly inserted.
validate-spacers: build validate-spacers-no-build

# Checks that the Kontrol summary dummy files have not been modified.
# If you have changed the summary files deliberately, update the hashes in the script.
# Use `openssl dgst -sha256` to generate the hash for a file.
check-kontrol-summaries-unchanged:
  ./scripts/checks/check-kontrol-summaries-unchanged.sh

# Runs semgrep on the contracts.
semgrep:
  cd ../../ && semgrep scan --config .semgrep/rules/ ./packages/contracts-bedrock

# Runs semgrep tests.
semgrep-test:
  cd ../../ && semgrep scan --test --config .semgrep/rules/ .semgrep/tests/

# Checks that the frozen code has not been modified.
check-frozen-code:
  ./scripts/checks/check-frozen-files.sh

# Runs all checks.
check:
  @just semgrep-test-validity-check \
  lint-check \
  snapshots-check-no-build \
  unused-imports-check-no-build \
  semver-diff-check-no-build \
  validate-deploy-configs \
  validate-spacers-no-build \
  interfaces-check-no-build \
  lint-forge-tests-check-no-build

########################################################
#                      DEV TOOLS                       #
########################################################

# Cleans, builds, lints, and runs all checks.
# Alias for pre-pr.
pre-commit *ARGS:
  just pre-pr {{ARGS}}

# Cleans, builds, lints, and runs all checks.
pre-pr *ARGS:
  #!/bin/bash
  # Optionally clean the previous build.
  # --clean is typically not needed but can force a clean build if you suspect cache issues.
  if [[ "{{ARGS}}" == *"--clean"* ]]; then
      just clean
  fi

  # Create temp directory for build cache if it doesn't exist.
  TEMP_BUILD_DIR=$(mktemp -d)
  trap '[ -d "$TEMP_BUILD_DIR" ] && rm -rf "$TEMP_BUILD_DIR"' EXIT INT

  just build-dev

  # Cache build artifacts for the dev build. We expect that developers will
  # generally be using the dev build, but a production build of the src
  # contracts is required for generating the correct snapshots. Production
  # build will overwrite the cache for the dev build, which ultimately means
  # that developers would have to wait a full minute for the dev build each
  # time they run this command. By caching the artifacts for the dev build, we
  # can save a lot of marginal time.
  cp -r artifacts "$TEMP_BUILD_DIR/"
  cp -r forge-artifacts "$TEMP_BUILD_DIR/"
  cp -r cache "$TEMP_BUILD_DIR/"

  just lint
  just build-source
  just check

  # Restore build artifacts after running checks.
  if [ -d "$TEMP_BUILD_DIR" ]; then
    cp -r "$TEMP_BUILD_DIR/artifacts" ./
    cp -r "$TEMP_BUILD_DIR/forge-artifacts" ./
    cp -r "$TEMP_BUILD_DIR/cache" ./
  fi

# Fixes linting errors.
lint-fix:
  forge fmt

# Fixes linting errors but doesn't fail if there are syntax errors. Useful for build command
# because the output of forge fmt can sometimes be difficult to understand but if there's a syntax
# error the build will fail anyway and provide more context about what's wrong.
lint-fix-no-fail:
  forge fmt || true

# Fixes linting errors and checks that the code is correctly formatted.
lint: lint-fix lint-check
