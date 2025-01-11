# Kurtosis Network Acceptance Tester (NAT)

* Performs a simple eth transfer on kurtosis-devnet.
* Currently hardcoded network and wallet configs to [kurtosis interop devnet](../kurtosis-devnet/tests/interop-devnet.json)

## Building and Running

1. ```just op-nat```
1. ```./bin/op-nat```

## TODOs

### PO

 1. Validate Balances before and after transfer
 1. Try an L1 Deposit to L2
   a. To do this must add optimism portal to network object

### P1

 1. Remove hardcodes / Read json file
