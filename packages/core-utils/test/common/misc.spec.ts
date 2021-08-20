import { expect } from '../setup'

/* Imports: Internal */
import { sleep } from '../../src'

describe('sleep', async () => {
  it('should return wait input amount of ms', async () => {
    const startTime = Date.now()
    await sleep(1000)
    const endTime = Date.now()
    expect(startTime + 1000 <= endTime).to.deep.equal(true)
  })
})
