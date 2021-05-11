/* External Imports */
import * as fs from 'fs'
import * as path from 'path'
import * as mkdirp from 'mkdirp'

const env = process.env
const CHAIN_ID = env.CHAIN_ID || '420'
const L2_CHUG_SPLASH_DEPLOYER_OWNER = env.L2_CHUG_SPLASH_DEPLOYER_OWNER || '0x' + '00'.repeat(20)

/* Internal Imports */
import { makeStateDump } from '../src/contract-dumps'
import { RollupDeployConfig } from '../src/contract-deployment'
;(async () => {
  const outdir = path.resolve(__dirname, '../dist/dumps')
  const outfile = path.join(outdir, 'state-dump.latest.json')
  mkdirp.sync(outdir)

  const config = {
    ovmGlobalContext: {
      ovmCHAINID: parseInt(CHAIN_ID, 10),
    },
    l2ChugSplashDeployerOwner: L2_CHUG_SPLASH_DEPLOYER_OWNER
  }

  const dump = await makeStateDump(config as RollupDeployConfig)
  fs.writeFileSync(outfile, JSON.stringify(dump, null, 4))
})()
