issues:
  ./ops/scripts/todo-checker.sh

# Runs semgrep on the entire monorepo.
semgrep:
  semgrep scan --config .semgrep/rules/ --error .

# Runs semgrep tests.
semgrep-test:
  semgrep scan --test --config .semgrep/rules/ .semgrep/tests/

lint-shellcheck:
  find . -type f -name '*.sh' -not -path '*/node_modules/*' -not -path './packages/contracts-bedrock/lib/*' -not -path './packages/contracts-bedrock/kout*/*' -exec sh -c 'echo "Checking $1"; shellcheck "$1"' _ {} \;

install-foundry:
  curl -L https://foundry.paradigm.xyz | bash && just update-foundry

update-foundry:
  bash ./ops/scripts/install-foundry.sh

check-foundry:
  bash ./ops/scripts/check-foundry.sh

install-kontrol:
  curl -L https://kframework.org/install | bash && just update-kontrol

update-kontrol:
  kup install kontrol --version v$(jq -r .kontrol < versions.json)

install-abigen:
  go install github.com/ethereum/go-ethereum/cmd/abigen@$(jq -r .abigen < versions.json)

print-abigen:
  abigen --version | sed -e 's/[^0-9]/ /g' -e 's/^ *//g' -e 's/ *$//g' -e 's/ /./g' -e 's/^/v/'

check-abigen:
  [[ $(just print-abigen) = $(cat versions.json | jq -r '.abigen') ]] && echo '✓ abigen versions match' || (echo '✗ abigen version mismatch. Run `just upgrade:abigen` to upgrade.' && exit 1)

upgrade-abigen:
  jq '.abigen = $v' --arg v $(just print:abigen) <<<$(cat versions.json) > versions.json

install-slither:
  pip3 install slither-analyzer==$(jq -r .slither < versions.json)

print-slither:
  slither --version

check-slither:
  [[ $(just print-slither) = $(jq -r .slither < versions.json) ]] && echo '✓ slither versions match' || (echo '✗ slither version mismatch. Run `just upgrade-slither` to upgrade.' && exit 1)

upgrade-slither:
  jq '.slither = $v' --arg v $(just print-slither) <<<$(cat versions.json) > versions.json

install-semgrep:
  pip3 install semgrep

print-semgrep:
  semgrep --version

check-semgrep:
  [ "$(just print-semgrep)" = "$(jq -r .semgrep < versions.json)" ] && echo '✓ semgrep versions match' || (echo '✗ semgrep version mismatch. Run `just upgrade-semgrep` to upgrade.' && exit 1)

upgrade-semgrep:
  jq '.semgrep = $v' --arg v $(just print-semgrep) <<<$(cat versions.json) > versions.json
