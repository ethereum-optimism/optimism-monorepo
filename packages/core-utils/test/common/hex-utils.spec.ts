import { expect } from '../setup'
import { BigNumber } from 'ethers'

/* Imports: Internal */
import {
  toRpcHexString,
  remove0x,
  add0x,
  fromHexString,
  toHexString,
  padHexString,
} from '../../src'

describe('remove0x', () => {
  it('should return undefined', () => {
    expect(remove0x(undefined)).to.deep.equal(undefined)
  })

  it('should return without a 0x', () => {
    const cases = [
      { input: '0x', output: '' },
      {
        input: '0x1f9840a85d5af5bf1d1762f925bdaddc4201f984',
        output: '1f9840a85d5af5bf1d1762f925bdaddc4201f984',
      },
      { input: 'a', output: 'a' },
    ]
    for (const test of cases) {
      expect(remove0x(test.input)).to.deep.equal(test.output)
    }
  })
})

describe('add0x', () => {
  it('should return undefined', () => {
    expect(add0x(undefined)).to.deep.equal(undefined)
  })

  it('should return with a 0x', () => {
    const cases = [
      { input: '0x', output: '0x' },
      {
        input: '1f9840a85d5af5bf1d1762f925bdaddc4201f984',
        output: '0x1f9840a85d5af5bf1d1762f925bdaddc4201f984',
      },
      { input: '', output: '0x' },
    ]
    for (const test of cases) {
      expect(add0x(test.input)).to.deep.equal(test.output)
    }
  })
})

describe('toHexString', () => {
  it('should return undefined', () => {
    expect(add0x(undefined)).to.deep.equal(undefined)
  })

  it('should return with a hex string', () => {
    const cases = [
      { input: 0, output: '0x00' },
      {
        input: '0',
        output: '0x30',
      },
      { input: '', output: '0x' },
    ]
    for (const test of cases) {
      expect(toHexString(test.input)).to.deep.equal(test.output)
    }
  })
})

describe('fromHexString', () => {
  it('should return a buffer from a hex string', () => {
    const cases = [
      { input: '0x', output: Buffer.from('', 'hex') },
      {
        input: '0x1f9840a85d5af5bf1d1762f925bdaddc4201f984',
        output: Buffer.from('1f9840a85d5af5bf1d1762f925bdaddc4201f984', 'hex'),
      },
      { input: '', output: Buffer.from('', 'hex') },
      {
        input: Buffer.from('1f9840a85d5af5bf1d1762f925bdaddc4201f984'),
        output: Buffer.from('1f9840a85d5af5bf1d1762f925bdaddc4201f984'),
      },
    ]

    for (const test of cases) {
      expect(fromHexString(test.input)).to.deep.equal(test.output)
    }
  })
})

describe('padHexString', () => {
  it('should return return input string if length is 2 + length * 2', () => {
    expect(padHexString('abcd', 1)).to.deep.equal('abcd')
    expect(padHexString('abcdefgh', 3).length).to.deep.equal(8)
  })
  it('should return a string padded with 0x and zeros', () => {
    expect(padHexString('0xabcd', 3)).to.deep.equal('0x00abcd')
  })
})

describe('toRpcHexString', () => {
  it('should parse 0', () => {
    expect(toRpcHexString(0)).to.deep.equal('0x0')
    expect(toRpcHexString(BigNumber.from(0))).to.deep.equal('0x0')
  })

  it('should parse non 0', () => {
    const cases = [
      { input: 2, output: '0x2' },
      { input: BigNumber.from(2), output: '0x2' },
      { input: 100, output: '0x64' },
      { input: BigNumber.from(100), output: '0x64' },
      { input: 300, output: '0x12c' },
      { input: BigNumber.from(300), output: '0x12c' },
    ]
    for (const test of cases) {
      expect(toRpcHexString(test.input)).to.deep.equal(test.output)
    }
  })
})
