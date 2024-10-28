#!/usr/bin/env bash
set -euo pipefail

# Runs the file unchanged script for each of the Kontrol summary files.
# Update these hashes if you have changed the summary files deliberately.
# Use `openssl dgst -sha256` to generate the hash for a file.
./scripts/checks/check-file-unchanged.sh ./test/kontrol/proofs/utils/DeploymentSummary.sol 73e1ed1f3c8bd9e46bb348b774443458206b2399e4eac303e2a92ac987eeefc6
./scripts/checks/check-file-unchanged.sh ./test/kontrol/proofs/utils/DeploymentSummaryCode.sol ffe7298024464a6a282ceb549db401cce4b36fe3342113250a6c674a3b4203ba
./scripts/checks/check-file-unchanged.sh ./test/kontrol/proofs/utils/DeploymentSummaryFaultProofs.sol 5ec8b19a00b47f15a2a8c8d632ab83933e19b0c2bbb04c905a2cda926e56038f
./scripts/checks/check-file-unchanged.sh ./test/kontrol/proofs/utils/DeploymentSummaryFaultProofsCode.sol 1215a561a2bffb5af62b532a2608c4048a22368e505f7ae1d237c16b4ef6a45e
