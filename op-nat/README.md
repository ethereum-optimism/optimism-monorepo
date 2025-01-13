# Network Acceptance Tester (NAT)


## Building and Running

1. `just op-nat`
1. `./bin/op-nat --kurtosis.devnet.manifest=../kurtosis-devnet/tests/interop-devnet.json`

## TODOs

### Test: simple-transfers

#### PO

 1. Validate Balances before and after transfer
 1. Try an L1 Deposit to L2
   a. To do this must add optimism portal to network object

#### P1

 1. Remove hardcodes / Read json file
 2. Aggregate results and output them to stdout


#### P2

 1. A nice way to add per-validator config/params