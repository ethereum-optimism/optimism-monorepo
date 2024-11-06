#!/bin/bash
set -euo pipefail

export FOUNDRY_PROFILE=kprove

SCRIPT_HOME="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
# shellcheck source=/dev/null
source "$SCRIPT_HOME/common.sh"
export RUN_KONTROL=true
parse_args "$@"

#############
# Functions #
#############
kontrol_build() {
  notif "Kontrol Build"
  # shellcheck disable=SC2086
  run kontrol build \
    --require $lemmas \
    --module-import $module \
    --no-metadata \
    ${rekompile} \
    ${regen}
  return $?
}

kontrol_prove() {
  notif "Kontrol Prove"
  # shellcheck disable=SC2086
  run kontrol prove \
    --max-depth $max_depth \
    --max-iterations $max_iterations \
    --smt-timeout $smt_timeout \
    --workers $workers \
    $reinit \
    $bug_report \
    $break_on_calls \
    $break_every_step \
    $tests \
    --init-node-from-diff $state_diff \
    --kore-rpc-command 'kore-rpc-booster --no-post-exec-simplify --equation-max-recursion 100 --equation-max-iterations 1000' \
    --xml-test-report \
    --maintenance-rate 16 \
    --assume-defined \
    --no-log-rewrites \
    --smt-timeout 16000 \
    --smt-retry-limit 0 \
    --no-stack-checks
  return $?
}

get_log_results(){
  RESULTS_FILE="results-$(date +'%Y-%m-%d-%H-%M-%S').tar.gz"
  LOG_PATH="test/kontrol/logs"
  RESULTS_LOG="$LOG_PATH/$RESULTS_FILE"

  if [ ! -d $LOG_PATH ]; then
    mkdir $LOG_PATH
  fi

  notif "Generating Results Log: $RESULTS_LOG"

  run tar -czf results.tar.gz kout-proofs/ > /dev/null 2>&1
  if [ "$LOCAL" = true ]; then
    mv results.tar.gz "$RESULTS_LOG"
  else
    docker cp "$CONTAINER_NAME:/home/user/workspace/results.tar.gz" "$RESULTS_LOG"
    # Check if kontrol_prove_report.xml exists in the container and copy it out if it does
    if docker exec "$CONTAINER_NAME" test -f /home/user/workspace/kontrol_prove_report.xml; then
      docker cp "$CONTAINER_NAME:/home/user/workspace/kontrol_prove_report.xml" "$LOG_PATH/kontrol_prove_report.xml"
      notif "Copied kontrol_prove_report.xml to $LOG_PATH"
    else
      notif "kontrol_prove_report.xml not found in container"
    fi
    tar -xzf "$RESULTS_LOG" > /dev/null 2>&1
  fi
  if [ -f "$RESULTS_LOG" ]; then
    cp "$RESULTS_LOG" "$LOG_PATH/kontrol-results_latest.tar.gz"
  else
    notif "Results Log: $RESULTS_LOG not found, skipping.."
  fi
  # Report where the file was generated and placed
  notif "Results Log: $(dirname "$RESULTS_LOG") generated"

  if [ "$LOCAL" = false ]; then
    notif "Results Log: $RESULTS_LOG generated"
    RUN_LOG="run-kontrol-$(date +'%Y-%m-%d-%H-%M-%S').log"
    docker logs "$CONTAINER_NAME" > "$LOG_PATH/$RUN_LOG"
    # Expand the tar folder to kout-proofs for Summary Results and caching
    tar -xzf "$RESULTS_LOG" -C "$WORKSPACE_DIR"  > /dev/null 2>&1
  fi
}

# Define the function to run on failure
on_failure() {
  get_log_results

  if [ "$LOCAL" = false ]; then
    clean_docker
  fi

  notif "Failure Cleanup Complete."
  exit 1
}

#########################
# kontrol build options #
#########################
# NOTE: This script has a recurring pattern of setting and unsetting variables,
# such as `rekompile`. Such a pattern is intended for easy use while locally
# developing and executing the proofs via this script. Comment/uncomment the
# empty assignment to activate/deactivate the corresponding flag
lemmas=test/kontrol/pausability-lemmas.md
base_module=PAUSABILITY-LEMMAS
module=OptimismPortalKontrol:$base_module
rekompile=--rekompile
# rekompile=
regen=--regen
# regen=

#################################
# Tests to symbolically execute #
#################################
# Temporarily unexecuted tests
# "OptimismPortalKontrol.prove_proveWithdrawalTransaction_paused0" \
# "OptimismPortalKontrol.prove_proveWithdrawalTransaction_paused1(" \
# "OptimismPortalKontrol.prove_proveWithdrawalTransaction_paused2" \
# "OptimismPortalKontrol.prove_proveWithdrawalTransaction_paused3" \
# "OptimismPortalKontrol.prove_proveWithdrawalTransaction_paused4" \
# "OptimismPortalKontrol.prove_proveWithdrawalTransaction_paused5" \
# "OptimismPortalKontrol.prove_proveWithdrawalTransaction_paused6" \
# "OptimismPortalKontrol.prove_proveWithdrawalTransaction_paused7" \
# "OptimismPortalKontrol.prove_proveWithdrawalTransaction_paused8" \
# "OptimismPortalKontrol.prove_proveWithdrawalTransaction_paused9" \
# "OptimismPortalKontrol.prove_proveWithdrawalTransaction_paused10" \

test_list=()
if [ "$SCRIPT_TESTS" == true ]; then
  test_list=(
    "OptimismPortal2Kontrol.prove_proveWithdrawalTransaction_paused0" \
    "OptimismPortal2Kontrol.prove_proveWithdrawalTransaction_paused1(" \
    "OptimismPortal2Kontrol.prove_proveWithdrawalTransaction_paused2" \
    "OptimismPortal2Kontrol.prove_proveWithdrawalTransaction_paused3" \
    "OptimismPortal2Kontrol.prove_proveWithdrawalTransaction_paused4" \
    "OptimismPortal2Kontrol.prove_proveWithdrawalTransaction_paused5" \
    "OptimismPortal2Kontrol.prove_proveWithdrawalTransaction_paused6" \
    "OptimismPortal2Kontrol.prove_proveWithdrawalTransaction_paused7" \
    "OptimismPortal2Kontrol.prove_proveWithdrawalTransaction_paused8" \
    "OptimismPortal2Kontrol.prove_proveWithdrawalTransaction_paused9" \
    "OptimismPortal2Kontrol.prove_proveWithdrawalTransaction_paused10" \
    "OptimismPortal2Kontrol.prove_finalizeWithdrawalTransaction_paused" \
    "L1StandardBridgeKontrol.prove_finalizeBridgeERC20_paused" \
    "L1StandardBridgeKontrol.prove_finalizeBridgeETH_paused" \
    "L1ERC721BridgeKontrol.prove_finalizeBridgeERC721_paused" \
    "L1CrossDomainMessengerKontrol.prove_relayMessage_paused"
  )
elif [ "$CUSTOM_TESTS" != 0 ]; then
  test_list=( "${@:${CUSTOM_TESTS}}" )
fi
tests=""
for test_name in "${test_list[@]}"; do
  tests+="--match-test $test_name "
done

#########################
# kontrol prove options #
#########################
max_depth=10000
max_iterations=10000
smt_timeout=100000
max_workers=16 # Set to 16 since there are 16 proofs to run
# workers is the minimum between max_workers and the length of test_list unless
# no test arguments are provided, in which case we default to max_workers
if [ "$CUSTOM_TESTS" == 0 ] && [ "$SCRIPT_TESTS" == false ]; then
  workers=${max_workers}
else
  workers=$((${#test_list[@]}>max_workers ? max_workers : ${#test_list[@]}))
fi
reinit=--reinit
reinit=
break_on_calls=--break-on-calls
break_on_calls=
break_every_step=--break-every-step
break_every_step=
bug_report=--bug-report
bug_report=
state_diff="./snapshots/state-diff/Kontrol-31337.json"

#############
# RUN TESTS #
#############
# Set up the trap to run the function on failure
trap on_failure ERR INT TERM
trap clean_docker EXIT
conditionally_start_docker

results=()
# Run kontrol_build and store the result
kontrol_build
results[0]=$?

# Run kontrol_prove and store the result
kontrol_prove
results[1]=$?

get_log_results

# Now you can use ${results[0]} and ${results[1]}
# to check the results of kontrol_build and kontrol_prove, respectively
if [ "${results[0]}" -ne 0 ] && [ "${results[1]}" -ne 0 ]; then
  echo "Kontrol Build and Prove Failed"
  exit 1
elif [ "${results[0]}" -ne 0 ]; then
  echo "Kontrol Build Failed"
  exit 1
elif [ "${results[1]}" -ne 0 ]; then
  echo "Kontrol Prove Failed"
  exit 2
else
  echo "Kontrol Passed"
fi

notif "DONE"
