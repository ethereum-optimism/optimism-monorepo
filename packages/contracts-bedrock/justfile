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

# Core forge build command
forge-build:
  forge build

# Builds the contracts.
build: lint-fix-no-fail forge-build interfaces-check-no-build

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

# Runs standard contract tests with rerun flag.
test-rerun: build-go-ffi
  forge test --rerun -vvv

# Run Kontrol tests and build all dependencies.
test-kontrol: build-go-ffi build kontrol-summary-full test-kontrol-no-build

# Run Kontrol tests without dependencies.
test-kontrol-no-build:
  ./test/kontrol/scripts/run-kontrol.sh script

# Runs contract coverage.
coverage: build-go-ffi
  forge coverage

# Runs contract coverage with lcov.
coverage-lcov: build-go-ffi
  forge coverage --report lcov


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

# Generates a gas snapshot without building.
gas-snapshot-no-build:
  forge snapshot --match-contract GasBenchMark --snap snapshots/.gas-snapshot

# Generates a gas snapshot.
gas-snapshot: build-go-ffi gas-snapshot-no-build

# Generates default Kontrol summary.
kontrol-summary:
  ./test/kontrol/scripts/make-summary-deployment.sh

# Generates fault proofs Kontrol summary.
kontrol-summary-fp:
  KONTROL_FP_DEPLOYMENT=true ./test/kontrol/scripts/make-summary-deployment.sh

# Generates all Kontrol summaries (default and FP).
kontrol-summary-full: kontrol-summary kontrol-summary-fp

# Generates ABI snapshots for contracts.
snapshots-abi-storage:
  go run ./scripts/autogen/generate-snapshots .

# Updates the snapshots/semver-lock.json file.
semver-lock:
  go run scripts/autogen/generate-semver-lock/main.go

# Generates core snapshots without building contracts. Currently just an alias for
# snapshots-abi-storage because we no longer run Kontrol snapshots here. Run
# kontrol-summary-full to build the Kontrol summaries if necessary.
snapshots-no-build: snapshots-abi-storage

# Builds contracts and then generates core snapshots.
snapshots: build snapshots-no-build


########################################################
#                        CHECKS                        #
########################################################

# Checks that the gas snapshot is up to date without building.
gas-snapshot-check-no-build:
  forge snapshot --match-contract GasBenchMark --snap snapshots/.gas-snapshot --check

# Checks that the gas snapshot is up to date.
gas-snapshot-check: build-go-ffi gas-snapshot-check-no-build

# Checks if the snapshots are up to date without building.
snapshots-check-no-build:
  ./scripts/checks/check-snapshots.sh --no-build

# Checks if the snapshots are up to date.
snapshots-check:
  ./scripts/checks/check-snapshots.sh

# Checks interface correctness without building.
interfaces-check-no-build:
  go run ./scripts/checks/interfaces

# Checks that all interfaces are appropriately named and accurately reflect the corresponding
# contract that they're meant to represent. We run "clean" before building because leftover
# artifacts can cause the script to detect issues incorrectly.2
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

# Runs all checks.
check:
  @just gas-snapshot-check-no-build \
  semgrep-test-validity-check \
  unused-imports-check-no-build \
  snapshots-check-no-build \
  lint-check \
  semver-diff-check-no-build \
  validate-deploy-configs \
  validate-spacers-no-build \
  interfaces-check-no-build \
  lint-forge-tests-check-no-build

########################################################
#                      DEV TOOLS                       #
########################################################

# Cleans, builds, lints, and runs all checks.
pre-pr: clean pre-pr-no-build

# Builds, lints, and runs all checks. Sometimes a bad cache causes issues, in which case the above
# `pre-pr` is preferred. But in most cases this will be sufficient and much faster then a full build.
pre-pr-no-build: build-go-ffi build lint gas-snapshot-no-build snapshots-no-build semver-lock check

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
