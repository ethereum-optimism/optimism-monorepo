import { HardhatUserConfig } from 'hardhat/types'
import 'solidity-coverage'
import * as dotenv from 'dotenv'

import {
  DEFAULT_ACCOUNTS_HARDHAT,
  RUN_OVM_TEST_GAS,
} from './test/helpers/constants'

// Hardhat plugins
import '@nomiclabs/hardhat-ethers'
import '@nomiclabs/hardhat-waffle'
import '@nomiclabs/hardhat-etherscan'
import 'hardhat-deploy'
import '@typechain/hardhat'
import '@eth-optimism/hardhat-ovm'
import './tasks/deploy'
import './tasks/l2-gasprice'
import './tasks/set-owner'
import './tasks/whitelist'
import './tasks/withdraw-fees'
import 'hardhat-gas-reporter'

// Load environment variables from .env
dotenv.config()

const enableGasReport = !!process.env.ENABLE_GAS_REPORT

const config: HardhatUserConfig = {
  networks: {
    hardhat: {
      accounts: DEFAULT_ACCOUNTS_HARDHAT,
      blockGasLimit: RUN_OVM_TEST_GAS * 2,
      live: false,
      saveDeployments: false,
      tags: ['local'],
      hardfork: 'istanbul',
    },
    // Add this network to your config!
    optimism: {
      url: 'http://127.0.0.1:8545',
      ovm: true,
      saveDeployments: false,
    },
  },
  mocha: {
    timeout: 50000,
  },
  solidity: {
    version: '0.7.6',
    settings: {
      optimizer: { enabled: true, runs: 200 },
      metadata: {
        bytecodeHash: 'none',
      },
      outputSelection: {
        '*': {
          '*': ['storageLayout'],
        },
      },
    },
  },
  ovm: {
    solcVersion: '0.7.6+commit.3b061308',
  },
  typechain: {
    outDir: 'dist/types',
    target: 'ethers-v5',
  },
  paths: {
    deploy: './deploy',
    deployments: './deployments',
  },
  namedAccounts: {
    deployer: {
      default: 0,
    },
  },
  gasReporter: {
    enabled: enableGasReport,
    currency: 'USD',
    gasPrice: 100,
    outputFile: process.env.CI ? 'gas-report.txt' : undefined,
  },
}

if (
  process.env.CONTRACTS_TARGET_NETWORK &&
  process.env.CONTRACTS_DEPLOYER_KEY &&
  process.env.CONTRACTS_RPC_URL
) {
  config.networks[process.env.CONTRACTS_TARGET_NETWORK] = {
    accounts: [process.env.CONTRACTS_DEPLOYER_KEY],
    url: process.env.CONTRACTS_RPC_URL,
    live: true,
    saveDeployments: true,
    tags: [process.env.CONTRACTS_TARGET_NETWORK],
  }
}

export default config
