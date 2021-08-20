import { expect } from '../setup'
import * as fees from '../../src/fees'
import { BigNumber, utils } from 'ethers'

const hundredBillion = 10 ** 11
const tenThousand = 10 ** 4

describe('Fees', () => {
  it('should count zeros and ones', () => {
    const cases = [
      { input: Buffer.from('0001', 'hex'), zeros: 1, ones: 1 },
      { input: '0x0001', zeros: 1, ones: 1 },
      { input: '0x', zeros: 0, ones: 0 },
      { input: '0x1111', zeros: 0, ones: 2 },
    ]

    for (const test of cases) {
      const [zeros, ones] = fees.zeroesAndOnes(test.input)
      zeros.should.eq(test.zeros)
      ones.should.eq(test.ones)
    }
  })

  describe('Rollup Fees', () => {
    const rollupFeesTests = [
      {
        name: 'simple',
        dataLen: 10,
        l1GasPrice: utils.parseUnits('1', 'gwei'),
        l2GasPrice: utils.parseUnits('1', 'gwei'),
        l2GasLimit: 437118,
      },
      {
        name: 'small-gasprices-max-gaslimit',
        dataLen: 10,
        l1GasPrice: utils.parseUnits('1', 'wei'),
        l2GasPrice: utils.parseUnits('1', 'wei'),
        l2GasLimit: 0x4ffffff,
      },
      {
        name: 'large-gasprices-max-gaslimit',
        dataLen: 10,
        l1GasPrice: utils.parseUnits('1', 'ether'),
        l2GasPrice: utils.parseUnits('1', 'ether'),
        l2GasLimit: 0x4ffffff,
      },
      {
        name: 'small-gasprices-max-gaslimit',
        dataLen: 10,
        l1GasPrice: utils.parseUnits('1', 'ether'),
        l2GasPrice: utils.parseUnits('1', 'ether'),
        l2GasLimit: 1,
      },
      {
        name: 'max-gas-limit',
        dataLen: 10,
        l1GasPrice: utils.parseUnits('5', 'ether'),
        l2GasPrice: utils.parseUnits('5', 'ether'),
        l2GasLimit: 99_970_000,
      },
      {
        name: 'zero-l2-gasprice',
        dataLen: 10,
        l1GasPrice: hundredBillion,
        l2GasPrice: 0,
        l2GasLimit: 196205,
      },
      {
        name: 'one-l2-gasprice',
        dataLen: 10,
        l1GasPrice: hundredBillion,
        l2GasPrice: 1,
        l2GasLimit: 196205,
      },
      {
        name: 'zero-l1-gasprice',
        dataLen: 10,
        l1GasPrice: 0,
        l2GasPrice: hundredBillion,
        l2GasLimit: 196205,
      },
      {
        name: 'one-l1-gasprice',
        dataLen: 10,
        l1GasPrice: 1,
        l2GasPrice: hundredBillion,
        l2GasLimit: 23255,
      },
      {
        name: 'zero-gasprices',
        dataLen: 10,
        l1GasPrice: 0,
        l2GasPrice: 0,
        l2GasLimit: 23255,
      },
      {
        name: 'larger-divisor',
        dataLen: 10,
        l1GasPrice: 0,
        l2GasLimit: 10,
        l2GasPrice: 0,
      },
    ]

    for (const test of rollupFeesTests) {
      it(`should pass for ${test.name} case`, () => {
        const data = Buffer.alloc(test.dataLen)
        const got = fees.TxGasLimit.encode({
          data,
          l1GasPrice: test.l1GasPrice,
          l2GasPrice: test.l2GasPrice,
          l2GasLimit: test.l2GasLimit,
        })

        const decoded = fees.TxGasLimit.decode(got)
        const roundedL2GasLimit = fees.ceilmod(test.l2GasLimit, 10_000)
        expect(decoded).to.deep.eq(roundedL2GasLimit)
      })
    }

    it('should decode numbers', () => {
      const x = 1
      expect(fees.TxGasLimit.decode(x)).to.deep.equal(
        BigNumber.from(x * tenThousand)
      )
    })
  })
})
