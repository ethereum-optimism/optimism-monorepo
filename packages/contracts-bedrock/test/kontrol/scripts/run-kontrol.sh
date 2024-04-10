#!/bin/bash
set -x

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
    --verbose \
    --require $lemmas \
    --module-import $module \
    $rekompile
  return $?
  # return 1 #Debugging
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
    $auto_abstract \
    $tests \
    $use_booster \
    --init-node-from $state_diff \
    --xml-test-report
  return $?
  # return 2 #Debugging
}

get_log_results(){
  RESULTS_FILE="results-$(date +'%Y-%m-%d-%H-%M-%S').tar.gz"
  LOG_PATH="test/kontrol/logs"
  RESULTS_LOG="$LOG_PATH/$RESULTS_FILE"

  if [ ! -d $LOG_PATH ]; then
    mkdir $LOG_PATH
  fi

  notif "Generating Results Log: $RESULTS_LOG"

  run tar -czvf results.tar.gz kout-proofs/ > /dev/null 2>&1
  if [ "$LOCAL" = true ]; then
    mv results.tar.gz "$RESULTS_LOG"
  else
    docker cp "$CONTAINER_NAME:/home/user/workspace/results.tar.gz" "$RESULTS_LOG"
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
    tar -xzvf "$RESULTS_LOG" -C "$WORKSPACE_DIR"
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
rekompile=
regen=--regen
# shellcheck disable=SC2034
regen=

#################################
# Tests to symbolically execute #
#################################

# Temporarily unexecuted tests
# "OptimismPortalKontrol.prove_proveWithdrawalTransaction_paused0" \
# "OptimismPortalKontrol.prove_proveWithdrawalTransaction_paused1" \
# "OptimismPortalKontrol.prove_proveWithdrawalTransaction_paused2" \
# "OptimismPortalKontrol.prove_proveWithdrawalTransaction_paused3" \
# "OptimismPortalKontrol.prove_proveWithdrawalTransaction_paused4" \
# "OptimismPortalKontrol.prove_proveWithdrawalTransaction_paused5" \
# "OptimismPortalKontrol.prove_proveWithdrawalTransaction_paused6" \
# "OptimismPortalKontrol.prove_proveWithdrawalTransaction_paused7" \
# "OptimismPortalKontrol.prove_proveWithdrawalTransaction_paused8" \
# "OptimismPortalKontrol.prove_proveWithdrawalTransaction_paused9" \
# "OptimismPortalKontrol.prove_proveWithdrawalTransaction_paused10" \
# "OptimismPortalKontrol.prove_proveWithdrawalTransaction_paused0" \
# "OptimismPortalKontrol.prove_proveWithdrawalTransaction_paused1" \
# "OptimismPortalKontrol.prove_finalizeWithdrawalTransaction_paused" \
# "L1StandardBridgeKontrol.prove_finalizeBridgeERC20_paused" \
# "L1StandardBridgeKontrol.prove_finalizeBridgeETH_paused" \
# "L1ERC721BridgeKontrol.prove_finalizeBridgeERC721_paused" \
# "L1CrossDomainMessengerKontrol.prove_relayMessage_paused"

# "DummyTest.prove_success" \
test_list=()
if [ "$SCRIPT_TESTS" == true ]; then
  test_list=(
              "DummyTest.prove_fail"
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
max_workers=7 # Set to 7 since the CI machine has 8 CPUs
# workers is the minimum between max_workers and the length of test_list
# unless no test arguments are provided, in which case we default to max_workers
if [ "$CUSTOM_TESTS" == 0 ] && [ "$SCRIPT_TESTS" == false ]; then
  workers=${max_workers}
else
  workers=$((${#test_list[@]}>max_workers ? max_workers : ${#test_list[@]}))
fi
reinit=--reinit
reinit=
break_on_calls=--no-break-on-calls
# break_on_calls=
break_every_step=--break-every-step
break_every_step=
auto_abstract=--auto-abstract-gas
auto_abstract=
bug_report=--bug-report
bug_report=
use_booster=--use-booster
# use_booster=
state_diff="./snapshots/state-diff/Kontrol-Deploy.json"

#############
# RUN TESTS #
#############
# Set up the trap to run the function on failure
# trap on_failure ERR INT TERM
# trap clean_docker EXIT
conditionally_start_docker

results=()
# Run kontrol_build and store the result
kontrol_build
results[0]=$?

# Run kontrol_prove and store the result
kontrol_prove
results[1]=$?

# Now you can use ${results[0]} and ${results[1]}
# to check the results of kontrol_build and kontrol_prove, respectively
if [ ${results[0]} -ne 0 ] && [ ${results[1]} -ne 0 ]; then
  echo "Kontrol Build and Prove Failed"
  exit 1
elif [ ${results[0]} -ne 0 ]; then
  echo "Kontrol Build Failed"
  exit 1
elif [ ${results[1]} -ne 0 ]; then
  echo "Kontrol Prove Failed"
  exit 2
  # Handle failure
else
  echo "Kontrol Passed"
  # Continue processing
fi

notif "DONE"
