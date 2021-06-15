import { predeploys } from '../../../src'
import { getStorageXOR } from '../constants'
import { ethers } from 'hardhat'

// These prestate values correspond to the storage slots in the SafetyChecker's `isSafeCodeHash` mapping.
// They were obtained using hardhat's console.log in the StateManager's getContractStorage() function.
// They can also be predicted using Solidity's storage layout, formula: keccak256(k . p),
// where k is the bytes32 codehash value, and p is the slot position (in this case bytes32(1)).
// The codehash values correspond to the creation and deployed bytecode of the various contracts which are
// deployed by other test-runner tests.
export const safetyCheckerPrestate = {
  StateManager: {
    contractStorage: {
      [predeploys.OVM_SafetyChecker]: {
        ['0x5b26a3fd2ea7ce31730a8ef708aaf7a0c9ae3f5179055dc2c176c069db1a973b']: getStorageXOR(
          ethers.constants.HashZero
        ),
        ['0x682f8199f8dafbf6ed36a88b49fe30d1345a0b265381e21ae1ed322ce403790f']: getStorageXOR(
          ethers.constants.HashZero
        ),
        ['0xf817571abebfe38a35d602c0cf654831de55d8c0a92475bb6158c96479b06c4b']: getStorageXOR(
          ethers.constants.HashZero
        ),
        ['0x80c1eeffdfb678098dbd8a28cda6f4a46c3e2b424239cfdac6ceae89f1e68ea0']: getStorageXOR(
          ethers.constants.HashZero
        ),
        ['0x019662b6c03adcefb9bbfaf547237f8b464a48d8898c191acc1be92bd60139fe']: getStorageXOR(
          ethers.constants.HashZero
        ),
        ['0x4b3dcd077968130c0e31da025bb27551489eefedc01a4ab0e8747b49e799c64b']: getStorageXOR(
          ethers.constants.HashZero
        ),
        ['0x747b7b59dd4415321ff8579bf74547553122694e6203d54b6affebe60e9de818']: getStorageXOR(
          ethers.constants.HashZero
        ),
        ['0x3496000adf91edfc042487ba74001d716a696902596685eb872e878f114287c9']: getStorageXOR(
          ethers.constants.HashZero
        ),
        ['0x7af4ce5fe0510bfff442bc7149708523058cb925886dc5b70f09cd1c653716f6']: getStorageXOR(
          ethers.constants.HashZero
        ),
        ['0x0d839b31aa4b526b4e6922b005a662495c290be3546686bcdbb9f389a7264ec7']: getStorageXOR(
          ethers.constants.HashZero
        ),
        ['0x6ec2eed2f0d804deef01046e50865606cc04c691b93b901c53ecde37c96d5d89']: getStorageXOR(
          ethers.constants.HashZero
        ),
        ['0x136a4a3e119b5432ac23aa88835bb063da759c0a009ef760976b98e00f3bf811']: getStorageXOR(
          ethers.constants.HashZero
        ),
        ['0x9e1c140426f6f1c30aa2b3bb4288dae893d57616de13178fe37f7c11235e5c74']: getStorageXOR(
          ethers.constants.HashZero
        ),
        ['0x9d49457f2e5aeddab140fa43726447305b24a26b7740f4d1947edff0aefa276d']: getStorageXOR(
          ethers.constants.HashZero
        ),
        ['0x0b54c49b38ef693df7a007f5e780b8aae19fbd708da3cf99429ff58d613d1280']: getStorageXOR(
          ethers.constants.HashZero
        ),
        ['0x64af05ca2fd59a71ccc45b05c8682cc7978a6031f7efd93ae6dbd3d5831fb11b']: getStorageXOR(
          ethers.constants.HashZero
        ),
        ['0x5f385a697af0e1a17c9f27d75025472aee3b96797155b95c142e17bf21530536']: getStorageXOR(
          ethers.constants.HashZero
        ),
        ['0x2e66ae5760bb855fedaf53bc7561a08ce5adf4fca145488958b1c96577ef2d14']: getStorageXOR(
          ethers.constants.HashZero
        ),
        ['0x47ba380bfdee7dd63501b4bd178729a8791481aa7ad7b37d97e18ca784d07511']: getStorageXOR(
          ethers.constants.HashZero
        ),
        ['0x622a0dced8f6023e8cdadd1dc17290592c439d53d575b93e25a919d84b288aa4']: getStorageXOR(
          ethers.constants.HashZero
        ),
        ['0x468cbcc2a935fecdcc9ebe235419e34259fd78d9ab8dc43cab47a4cb191262c0']: getStorageXOR(
          ethers.constants.HashZero
        ),
        ['0xf4bf285dfa3f1dde6c28345116102707c04285ee1136eebba60ae31e9b9c834d']: getStorageXOR(
          ethers.constants.HashZero
        ),
        ['0x3cf65acbff73964b0c6d6d4870651aa6b435eb3ecaef9dc8e5e3cade8faa34f1']: getStorageXOR(
          ethers.constants.HashZero
        ),
        ['0xbb807538060b504e9b76bc9ae91da101beb34f0e81e720f2d2090ccbedfb4fd1']: getStorageXOR(
          ethers.constants.HashZero
        ),
        ['0xdeaf16f2ea089195a60249d6c7c3b7944a6f5e16d8e705dbe92507555abff54d']: getStorageXOR(
          ethers.constants.HashZero
        ),
        ['0x5cb012338d5eb4cc2760b3d285423883e8f60b8126f840fdc9a347049cd5837a']: getStorageXOR(
          ethers.constants.HashZero
        ),
      },
    },
    verifiedContractStorage: {
      [predeploys.OVM_SafetyChecker]: {
        ['0x5b26a3fd2ea7ce31730a8ef708aaf7a0c9ae3f5179055dc2c176c069db1a973b']: true,
        ['0x682f8199f8dafbf6ed36a88b49fe30d1345a0b265381e21ae1ed322ce403790f']: true,
        ['0xf817571abebfe38a35d602c0cf654831de55d8c0a92475bb6158c96479b06c4b']: true,
        ['0x80c1eeffdfb678098dbd8a28cda6f4a46c3e2b424239cfdac6ceae89f1e68ea0']: true,
        ['0x019662b6c03adcefb9bbfaf547237f8b464a48d8898c191acc1be92bd60139fe']: true,
        ['0x4b3dcd077968130c0e31da025bb27551489eefedc01a4ab0e8747b49e799c64b']: true,
        ['0x747b7b59dd4415321ff8579bf74547553122694e6203d54b6affebe60e9de818']: true,
        ['0x3496000adf91edfc042487ba74001d716a696902596685eb872e878f114287c9']: true,
        ['0x7af4ce5fe0510bfff442bc7149708523058cb925886dc5b70f09cd1c653716f6']: true,
        ['0x0d839b31aa4b526b4e6922b005a662495c290be3546686bcdbb9f389a7264ec7']: true,
        ['0x6ec2eed2f0d804deef01046e50865606cc04c691b93b901c53ecde37c96d5d89']: true,
        ['0x136a4a3e119b5432ac23aa88835bb063da759c0a009ef760976b98e00f3bf811']: true,
        ['0x9e1c140426f6f1c30aa2b3bb4288dae893d57616de13178fe37f7c11235e5c74']: true,
        ['0x9d49457f2e5aeddab140fa43726447305b24a26b7740f4d1947edff0aefa276d']: true,
        ['0x0b54c49b38ef693df7a007f5e780b8aae19fbd708da3cf99429ff58d613d1280']: true,
        ['0x64af05ca2fd59a71ccc45b05c8682cc7978a6031f7efd93ae6dbd3d5831fb11b']: true,
        ['0x5f385a697af0e1a17c9f27d75025472aee3b96797155b95c142e17bf21530536']: true,
        ['0x2e66ae5760bb855fedaf53bc7561a08ce5adf4fca145488958b1c96577ef2d14']: true,
        ['0x47ba380bfdee7dd63501b4bd178729a8791481aa7ad7b37d97e18ca784d07511']: true,
        ['0x622a0dced8f6023e8cdadd1dc17290592c439d53d575b93e25a919d84b288aa4']: true,
        ['0x468cbcc2a935fecdcc9ebe235419e34259fd78d9ab8dc43cab47a4cb191262c0']: true,
        ['0xf4bf285dfa3f1dde6c28345116102707c04285ee1136eebba60ae31e9b9c834d']: true,
        ['0x3cf65acbff73964b0c6d6d4870651aa6b435eb3ecaef9dc8e5e3cade8faa34f1']: true,
        ['0xbb807538060b504e9b76bc9ae91da101beb34f0e81e720f2d2090ccbedfb4fd1']: true,
        ['0xdeaf16f2ea089195a60249d6c7c3b7944a6f5e16d8e705dbe92507555abff54d']: true,
        ['0x5cb012338d5eb4cc2760b3d285423883e8f60b8126f840fdc9a347049cd5837a']: true,
      },
    },
  },
}
