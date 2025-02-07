# op-dripper

A service designed to execute Drippie drips from a given EOA. It will only trigger based on the configured drips of the passed drippie address.

### Required Configuration

The main configuration for the EOA, Drippie contract to trigger, and the Ethereum L1 RPC.

- `OP_DRIPPER_DRIPPIE_ADDRESS`: The address of the Drippie contract to interact with
- `OP_DRIPPER_L1_ETH_RPC`: RPC URL for the L1 Ethereum chain
- Authentication (choose one):
  - `OP_DRIPPER_PRIVATE_KEY`: Private key for the executing EOA
  - `OP_DRIPPER_MNEMONIC`: Mnemonic phrase (with optional `OP_DRIPPER_HD_PATH`)

### Transaction Settings

Settings to configure to ensure transactions are landing / the service is keeping up with new blocks.

- `OP_DRIPPER_NUM_CONFIRMATIONS` (default: 10): Number of confirmations to wait after sending a transaction
- `OP_DRIPPER_POLL_INTERVAL` (default: 12s): Frequency to poll for new blocks
- `OP_DRIPPER_RESUBMISSION_TIMEOUT` (default: 48s): Duration before transaction resubmission
- `OP_DRIPPER_TXMGR_MIN_BASEFEE` (default: 1): Minimum base fee in gwei
- `OP_DRIPPER_TXMGR_MIN_TIP_CAP` (default: 1): Minimum tip cap in gwei

## Basic Usage

Basic service with no transaction configuration

```bash
op-dripper --drippie-address=0x... --l1-eth-rpc=https://... --private-key=0x...
```

