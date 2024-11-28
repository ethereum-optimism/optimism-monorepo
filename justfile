# Checks that TODO comments have corresponding issues.
todo-checker:
  ./ops/scripts/todo-checker.sh

# Runs semgrep on the entire monorepo.
semgrep:
  semgrep scan --config .semgrep/rules/ --error .

# Runs semgrep tests.
semgrep-test:
  semgrep scan --test --config .semgrep/rules/ .semgrep/tests/

# Runs shellcheck.
shellcheck:
  find . -type f -name '*.sh' -not -path '*/node_modules/*' -not -path './packages/contracts-bedrock/lib/*' -not -path './packages/contracts-bedrock/kout*/*' -exec sh -c 'echo "Checking $1"; shellcheck "$1"' _ {} \;

########################################################
#                 DEPENDENCY MANAGEMENT                #
########################################################

# Generic task for checking if a tool version is up to date.
check-tool-version tool:
  #!/usr/bin/env bash
  EXPECTED=$(jq -r .{{tool}} < versions.json)
  ACTUAL=$(just print-{{tool}})
  if [ "$ACTUAL" = "$EXPECTED" ]; then
    echo "✓ {{tool}} versions match"
  else
    echo "✗ {{tool}} version mismatch (expected $EXPECTED, got $ACTUAL), run 'just install-{{tool}}' to upgrade"
    exit 1
  fi

# Installs foundry
install-foundry:
  bash ./ops/scripts/install-foundry.sh

# Prints current foundry version.
print-foundry:
  forge --version

# Checks if installed foundry version is correct.
check-foundry:
  bash ./ops/scripts/check-foundry.sh

# Installs correct kontrol version.
install-kontrol:
  bash ./ops/scripts/install-kontrol.sh

# Prints current kontrol version.
print-kontrol:
  kontrol version

# Checks if installed kontrol version is correct.
check-kontrol:
  just check-tool-version kontrol

# Installs correct abigen version.
install-abigen:
  go install github.com/ethereum/go-ethereum/cmd/abigen@$(jq -r .abigen < versions.json)

# Prints current abigen version.
print-abigen:
  abigen --version | sed -e 's/[^0-9]/ /g' -e 's/^ *//g' -e 's/ *$//g' -e 's/ /./g' -e 's/^/v/'

# Checks if installed abigen version is correct.
check-abigen:
  just check-tool-version abigen

# Installs correct slither version.
install-slither:
  pip3 install slither-analyzer==$(jq -r .slither < versions.json)

# Prints current slither version.
print-slither:
  slither --version

# Checks if installed slither version is correct.
check-slither:
  just check-tool-version slither

# Installs correct semgrep version.
install-semgrep:
  pip3 install semgrep=="$(jq -r .semgrep < versions.json)"

# Prints current semgrep version.
print-semgrep:
  semgrep --version | head -n 1

# Checks if installed semgrep version is correct.
check-semgrep:
  just check-tool-version semgrep

# Installs correct go version.
install-go:
  echo "error: go must be installed manually" && exit 1

# Prints current go version.
print-go:
  go version | sed -E 's/.*go([0-9]+\.[0-9]+\.[0-9]+).*/\1/'

# Checks if installed go version is correct.
check-go:
  just check-tool-version go
