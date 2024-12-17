# Supertoken advanced testing

## Note

This campaign will need to be updated the redesign `OptimismSuperchainERC20` redesign. Please delete this comment once the update is done.

## Milestones

The supertoken ecosystem consists of not just the supertoken contract, but the required changes to other contracts for liquidity to reach the former.

Considering only the supertoken contract is merged into the `develop` branch, and work for the other components is still in progress, we define three milestones for the testing campaign:

- SupERC20: concerned with only the supertoken contract, the first one to be implemented
- Factories: covers the above + the development of `OptimismSuperchainERC20Factory` and required changes to `OptimismMintableERC20Factory`
- Liquidity Migration: includes the `convert` function on the `L2StandardBridgeInterop` to migrate liquidity from legacy tokens into supertokens

## Definitions

- _legacy token:_ an OptimismMintableERC20 or L2StandardERC20 token on the superchain that has either been deployed by the factory after the liquidity migration upgrade to the latter, or has been deployed before it **but** added to factory’s `deployments` mapping as part of the upgrade. This testing campaign is not concerned with tokens on L1 or not listed in the factory’s `deployments` mapping.
- _supertoken:_ a SuperchainERC20 contract deployed by the `OptimismSuperchainERC20Factory`

# Ecosystem properties

legend:

- `[ ]`: property not yet tested
- `**[ ]**`: property not yet tested, dev/research team has asked for extra focus on it
- `[X]`: tested/proven property
- `[~]`: partially tested/proven property
- `:(`: property won't be tested due to some limitation

## Unit test

| id  | milestone           | description                                                                                | tested |
| --- | ------------------- | ------------------------------------------------------------------------------------------ | ------ |
| 0   | Factories           | supertoken token address does not depend on the executing chain’s chainID                  | [ ]    |
| 1   | Factories           | supertoken token address depends on remote token, name, symbol and decimals                | [ ]    |
| 2   | Liquidity Migration | convert() should only allow converting legacy tokens to supertoken and viceversa           | [ ]    |
| 3   | Liquidity Migration | convert() only allows migrations between tokens representing the same remote asset         | [ ]    |
| 4   | Liquidity Migration | convert() only allows migrations from tokens with the same decimals                        | [ ]    |
| 5   | Liquidity Migration | convert() burns the same amount of legacy token that it mints of supertoken, and viceversa | [ ]    |
| 25  | SupERC20            | supertokens can't be reinitialized                                                         | [x]    |

## Valid state

| id  | milestone | description                                                                    | tested |
| --- | --------- | ------------------------------------------------------------------------------ | ------ |
| 6   | SupERC20  | calls to sendERC20 succeed as long as caller has enough balance                | []     |
| 7   | SupERC20  | calls to relayERC20 always succeed as long as the cross-domain caller is valid | []     |

## Variable transition

| id  | milestone           | description                                                                                       | tested |
| --- | ------------------- | ------------------------------------------------------------------------------------------------- | ------ |
| 8   | SupERC20            | sendERC20 with a value of zero does not modify accounting                                         | []     |
| 9   | SupERC20            | relayERC20 with a value of zero does not modify accounting                                        | []     |
| 10  | SupERC20            | sendERC20 decreases the token's totalSupply in the source chain exactly by the input amount       | []     |
| 26  | SupERC20            | sendERC20 decreases the sender's balance in the source chain exactly by the input amount          | []     |
| 27  | SupERC20            | relayERC20 increases sender's balance in the destination chain exactly by the input amount        | [x]    |
| 11  | SupERC20            | relayERC20 increases the token's totalSupply in the destination chain exactly by the input amount | [ ]    |
| 12  | Liquidity Migration | supertoken total supply only increases on calls to mint() by the L2toL2StandardBridge             | [~]    |
| 13  | Liquidity Migration | supertoken total supply only decreases on calls to burn() by the L2toL2StandardBridge             | [ ]    |
| 14  | SupERC20            | supertoken total supply starts at zero                                                            | [x]    |
| 15  | Factories           | deploying a supertoken registers its remote token in the factory                                  | [ ]    |
| 16  | Factories           | deploying an OptimismMintableERC20 registers its remote token in the factory                      | [ ]    |

## High level

| id  | milestone           | description                                                                                                                                                           | tested |
| --- | ------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ |
| 17  | Liquidity Migration | only calls to convert(legacy, super) can increase a supertoken’s total supply across chains                                                                           | [ ]    |
| 18  | Liquidity Migration | only calls to convert(super, legacy) can decrease a supertoken’s total supply across chains                                                                           | [ ]    |
| 19  | Liquidity Migration | sum of supertoken total supply across all chains is always <= to convert(legacy, super)- convert(super, legacy)                                                       | [~]    |
| 20  | SupERC20            | tokens sendERC20-ed on a source chain to a destination chain can be relayERC20-ed on it as long as the source chain is in the dependency set of the destination chain | [ ]    |
| 21  | Liquidity Migration | sum of supertoken total supply across all chains is = to convert(legacy, super)- convert(super, legacy) when all cross-chain messages are processed                   | [~]    |

## Atomic bridging pseudo-properties

As another layer of defense, the following properties are defined which assume bridging operations to be atomic (that is, the sequencer and L2Inbox and CrossDomainMessenger contracts are fully abstracted away, `sendERC20` triggering the `relayERC20` call on the same transaction)
It’s worth noting that these properties will not hold for a live system

| id  | milestone           | description                                                                                                                        | tested |
| --- | ------------------- | ---------------------------------------------------------------------------------------------------------------------------------- | ------ |
| 22  | SupERC20            | sendERC20 decreases sender balance in source chain and increases receiver balance in destination chain exactly by the input amount | []     |
| 23  | SupERC20            | sendERC20 decreases total supply in source chain and increases it in destination chain exactly by the input amount                 | []     |
| 24  | Liquidity Migration | sum of supertoken total supply across all chains is always equal to convert(legacy, super)- convert(super, legacy)                 | [~]    |
