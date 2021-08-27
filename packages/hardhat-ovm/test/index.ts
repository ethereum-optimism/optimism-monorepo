import path from 'path'
import * as fsExtra from 'fs-extra'
import { assert } from 'chai'
import { resetHardhatContext } from 'hardhat/plugins-testing'
import { HardhatRuntimeEnvironment } from 'hardhat/types'

const hardhatProjectPath = path.join(__dirname, 'hardhat-project')

describe('Optimism plugin', () => {
  let hre: HardhatRuntimeEnvironment

  before('set environment to the hardhat-project folder', async () => {
    process.chdir(hardhatProjectPath)
  })

  afterEach('reset hardhat runtime environment', () => {
    resetHardhatContext()
  })

  describe('when compiling', () => {
    describe('evm artifacts', () => {
      before('remove cache and artifacts', async () => {
        await fsExtra.remove(path.join(hardhatProjectPath, 'artifacts'))
        await fsExtra.remove(path.join(hardhatProjectPath, 'cache'))
      })

      before('target the localhost network', () => {
        process.env.HARDHAT_NETWORK = 'localhost'
        hre = require('hardhat')
      })

      before('compile', async () => {
        await hre.run('compile', { quiet: true })
      })

      it('creates a cache folder', async () => {
        assert.ok(
          await fsExtra.pathExists(path.join(hardhatProjectPath, 'cache'))
        )
      })

      it('creates an artifacts folder', async () => {
        assert.ok(
          await fsExtra.pathExists(path.join(hardhatProjectPath, 'artifacts'))
        )
      })
    })

    describe('ovm artifacts', () => {
      before('remove cache and artifacts', async () => {
        await fsExtra.remove(path.join(hardhatProjectPath, 'artifacts-ovm'))
        await fsExtra.remove(path.join(hardhatProjectPath, 'cache-ovm'))
      })

      before('target the optimism network', () => {
        process.env.HARDHAT_NETWORK = 'optimism'
        hre = require('hardhat')
      })

      before('compile', async () => {
        await hre.run('compile', { quiet: true })
      })

      it('creates a cache-ovm folder', async () => {
        assert.ok(
          await fsExtra.pathExists(path.join(hardhatProjectPath, 'cache-ovm'))
        )
      })

      it('creates an artifacts-ovm folder', async () => {
        assert.ok(
          await fsExtra.pathExists(
            path.join(hardhatProjectPath, 'artifacts-ovm')
          )
        )
      })
    })
  })
})
