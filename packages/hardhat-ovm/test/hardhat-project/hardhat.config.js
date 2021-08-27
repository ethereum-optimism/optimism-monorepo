require('../../dist/index')

module.exports = {
  solidity: '0.7.6',
  networks: {
    optimism: {
      url: 'http://locahost:8545',
      ovm: true,
    },
  },
}
