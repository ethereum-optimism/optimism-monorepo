package node

import (
	"context"
	"encoding/json"
	"math/rand"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-node/metrics"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/version"
	rpcclient "github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
)

func TestOutputAtBlock(t *testing.T) {
	log := testlog.Logger(t, log.LevelError)

	// Test data for Merkle Patricia Trie: proof the eth2 deposit contract account contents (mainnet).
	headerTestData := `
	{
		"parentHash": "0x47e0bb8a195bb8c41f88451ebb6c6e19caea3538e259c4f8f576f563651b2ea0",
		"sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
		"miner": "0x3ecef08d0e2dad803847e052249bb4f8bff2d5bb",
		"stateRoot": "0xb46d4bcb0e471e1b8506031a1f34ebc6f200253cbaba56246dd2320e8e2c8f13",
		"transactionsRoot": "0x51cb26cf4c43af5dcc4188aa75880f4d3287ceb2ed386a45eb3ac03cd1e9af1b",
		"receiptsRoot": "0xc162238f66ce50a32f2f28e704bff473ec3e24f40ac78951de228712fd70aae0",
		"logsBloom": "0x4171800201004021001804029c02602220000484a2105822038000028010441800061a4444822145e002000cc30505848be96119a82220406240104b0a652018450d00090018104848430009493171202140a04081440048180000408040108002508d4002fa40010880110008018810902989f00d81040080210430c00864003a108042000000040108001a400020001934a6890b20828c600901c020180084020800051120a806202900989e2280005310024038808019a08025e2a09040000029824340600a2820215040200e044144408052cd0a4c320441a146100260002838a2180300040294100480215488a050e2420a2480a1420480085441222810",
		"difficulty": "0x2eeba6b1f2d375",
		"number": "0xdcdc89",
		"gasLimit": "0x1c9c380",
		"gasUsed": "0x4b7cf4",
		"timestamp": "0x62419993",
		"extraData": "0x73656f36",
		"mixHash": "0x91af27781efde0b9a52631b7770a1ba3cb789e2bbf02bcf4538d22bfed01158e",
		"nonce": "0x59ad2bebfd070533",
		"baseFeePerGas": "0x59eab8ea2",
		"hash": "0x8512bee03061475e4b069171f7b406097184f16b22c3f5c97c0abfc49591c524"
	}`
	var header types.Header
	err := json.Unmarshal([]byte(headerTestData), &header)
	assert.NoError(t, err)

	resultTestData := `
	{
		"address": "0x00000000219ab540356cbb839cbe05303d7705fa",
		"accountProof": [
			"0xf90211a0053de2c69b88d64fcbb62d9da3282c7100d4b87ae1fb2577c07f0a9e25c80991a0675e4b40d962dce5bf03e24da87d193dbe99a65c1b26d1d6f8738222ccb953c6a05c5479c870b639b36fd6e4c3014f6250bb961b8312775bad0e6a605e1e9c9f55a0087c6656d467c8bffdc00ad447e6b2be7e9e173139597f8e3db628a31505497fa00a2a6f22504a5a4ebff8fd869e781ef24ab657e64ce4e6ef0228ea9ebb6283f7a0ca22287cb61d05a6f39fbf62a92ae7ffbad20102ba6462261866008d3930c8c8a00d5899983ed06e619dd6fdd6a9b678a3da6ffebf62debedc6981ea7c41934a37a02b7efb0aa93b02ed4c232a6d420c3f772ef915bc71397b98c4d128847058fc95a0d018a365d4c1eaa02c7f63153bda7dbbf66fc3e40b51f1bc2f9c9bcc7e8020d1a0eff9b494995139443a09365e928a74f36cc2cca2f0f675f3df530f65c4e6470ea012c7419fe80ec73ffc5ef2c9839593e2dec3e6911d21db20b2323e5f6801417ea09db162242bc6382a6fb0dce195157c8bf47c13ebcc9506dcf1b466a1ff3bfe59a0f96c17b003d5ec293f5332fb830bc34667b396dcd3d4e2ed508ff77d965f78c5a04099fe09f64b53cdb90f3537a10c5b1f8f6e8dfa2a4308acdbd6b3496629869ea0efd2b1a33d4562cab8c20748fb3bdb60aabd85cd6c112e826738af3a3bcb7b3ca03b701015938a78fca54055e8797fdbe2b63e029e3d88e519d81e4aa74f52516c80",
			"0xf90211a07a61b559adab3b69960d88a06052127b6c4e1f052adaa714a78a94cf77db6bdda00773c97a11c32dbe5f6d5dc2bf4e4cc25bf0408a3cb5fe54bb7f65ad548eb08fa0278563f7e29d7edfccb56a1da17f2e171f28eac51e3e4b0b425c0e8472a5686ba0893e1be872339b57d89d3741df456d9a91754a00ee080aa7aa175f674f57c84da0c523ed9cfe7927f8ec7e47a65155a69d77c0e9485b50d52d240cf6836e3a02a4a07c9e0b7c24c780fc2657d2f902ccaa749ac284c3ac7c192d1c6509bcf858a536a05595963f4d1e353e660d79382b41681d7e006af420dae1c0de7fc22e1b9df86ca0d299e02df563fa2904626a4ed6de01b0bffb49204885cc9e82bb04348bb87e63a069e72616ce71f8b72cbd7b37eefab216fa3b9324947d0870ff1e133b93b74818a01caf32199ac1573b5f8ceb82b454424fab10fce895544f1eb7e327c94f0a235ea0795525db25d2453b0c41e3fe939b4fbca046820c7b498736cc6f98a9afc6b56aa0f41cca6a5e1791eccd77c12318ea9a8d7fff643d84db7b716abda7e2b4fffdf1a0f6e9e0abfbe843102ad697567ae36c3c1486ec167956a4e149cce9da89980d2da0a48b1793b3deb902a3d35d7c98528c37005495f252e46ef06e7cba54e17ad638a0c4a38db3d5324e46f18ede4bfbe566932fa8cc8fa7891eac5b03c751a72ee65da01199874c07a3e9234f54158d49fe26e0eb9e174f3a245a10b0bb399e715ef73e80",
			"0xf90211a0151a549c4bda6b7ad536eb85a0955cfdc9baef3859722a02641b4995a765e039a00d6c2898c6f9c5c5cbb225e5ce25092f8214da069847fcc92d2d5cd262abd426a08cf5d2ec077fb3c36df58d7cbcd5c7245de7de6cbf0faea7879c07210e2178e4a0991e0d147c3d0b0257509ed8ecb7d46d817287823a2c7632d7e545a07e5c05efa0650dd56a943e6eabbc57507a843a81fc049de047d6194606ed29b3abf3b8fb98a0814c4a99d93d88f88033ca3813f37e4476b3be1a8a20f2b387ed2af666014843a090c8ce86b3e8bb37bb41bbceac49a851feaf0a7d7f958d3733d46c35321d6113a03a59be04ecd3bd7ef287d55ca44eba754ceb73b11984eb07f5c9ef662473e264a0b8dcabc2461c7aa0d5e9e64c00471c866c61221ba12abd7230d1cf6363074d8aa01c822a721bbdf3a25cfc5c039a2203d7dde065077d8e9e2a79d785634049651da0956f1b89b07519c33567bf334ed83b22ee76ef5b057831f52c227bf87b12e7d4a0f5bc6aacd26c0cfe7e6854cc61ef085195e7ecf5f04a656272eaaca0910a570ba00538ca73976dc9d42683bfd6c81f85fffe7594532b2f2d60f035c7662ee636f3a0481681e232913e57fc0dcdf3e41558726c475bd824efd190e87c4cc6c59c5abfa0421a065bd09dd47510c9b5f05bdcce6992f8f290252ff5ac039ec3b74b784b54a06fecfc2bb7fb3fddd8988453f1687e4c7eefa73fae5b23a8a6c00c6c2347c70780",
			"0xf90211a030229b7cca8cc53d7edd465792b917c92da8a54e9ab1dd2fbe13c1952f49bb15a0d8ce8468603b262264ae9c1086f98a8f6cf9b89bf9b08c7e03c7e3d78a1e28afa0f0874b64554052fb583cea8da9939bd8b6f6f083a15424dd3613bdaabccd723ca0a9293e5b4cc2cf664296a87b3bdb9ad066af00b425a2efb29dfdda2c6d2b5b7ba0e19e1cd86832a1998da1c117a1ba38634de7030f7f396a3e1728bee5953feabca02f0c836b4fe1536c4ec538857318355dd2b98c71e3f11244bcb62d9a77f53a9aa0b3891659442e5da4b5a87bc30e6d646f14ddf99aac6ead34d2dd0929b425650ca073861564bc6b774edce16d69fef0209c1ae6cc7c7ae9abf66aa22ebff6db3baca09c2bc83919d84f12158f0fb3075107fe29d9e9f0e1225676f72e9119f4db3ea2a0751f8378a2e268d8bf15f572061dd8f50090156af8ad210143f9fb434ec3314ca0f3710dbc5a154804c31b7390f681e4ce7569350ebccaa1763c644781d8afc4c8a082295baf1fb8f3c98c52554b95a08bc5457b0fdc936a1d6ae69aa3316388c568a097ca8b1bdfbc6b0156a2ff293f4bdfe421dabcf9634ccc12d2ba399020ec3027a0302946c9212085e56c22ad229a87fba0b5c728f5904b1ed5e905fcfba3c83f09a032e8579104775cc6ebed949b21d3afd1a6ff9d66c3377384b147ebc99b4d3780a0c26e0c54ec91c56c6bd4a84029ad24fb890635c51df16fd1d56a6d83d0dca81680",
			"0xf90211a0dd0f9c581d9abf2b4d97e6540f3026ad0c84fd32c77ca28178bc345f095ee8a0a0743c851689b4bf826b25307c8b0af143fa5ed754cb54b6365f6db0b43178a49fa0fca51828e9a618deac1de3ae0f3f8ac851bc26386c41a279cc43236b22d636eca0be49b0fd047089e186855a6d18c3b70399204c01a612bd7e7ff447999b188484a0fe48aeb769431c737ed50395843234d6bd2ed2c6e8be916df4f1724981675810a087c33eebdeece82fa8a21649b6c3b1e9fcd3de4d5bb68729433deb7e32e87481a083226a8b46c513232ab509daa733ffa1573b9763b0a1f7e8915fe98e0e69e358a07b0ee3cc203cc3ece1cb4b1714d1cb01224ec6244101ff77f599609798efaecca088c32b6ccc3c1afb2e1d5a4df69089cfca7351bc171b7f8bf4b52d2e2588cba6a0204d7c392ed55dd9576ba8c6ecce8affadd967a1bc62141922fecab72bbf4907a00d8cf034eeb5f9686c3ebeacde2ac4eef1fefd9a2006ff8f144207e874da70c6a0637929a730614ab1f0b780c5bef785afba18e12f0ea283789cb66fe6923f4278a08374de3370417c480be77f025eee79151f73ded8d071b518e5b258123d923af1a0d3f27cd43be2c58b528372c9187b99a49a8d06f504ffa2d5ff4cd3ec74bd3ccca02104bbd4bee7770c4663e95ec8881005062b77324b436812e399b44c93961d7fa035847ce3af7e94228ab92d86a0fbb23ba5b6a1f8ced7779eafcfe6b8da466d0680",
			"0xf90211a078ec3c13d353c11178ffa862501bf35e40e36bc86f396dac2e17602a0c747d5ba0d851ff649a0d78647807f486a934a35fc9e41ddbb64a09bbebbe205abd338ee8a019d1ce172e5a45e3dc0866eb071e38a13338ae6dbbfdc70aad3b2f82cc072f8fa062c437592bd2721d81a7197318c91b103c6d568a9746d3a1c806ed6370271fc1a056507388b75afefff70474a547d48d53ebe1eff4916af8a712fbd012d9b6c07ca05038b123df05284a4aa84e4f1bea52da64b7d3ee155817580901846606963669a032547a9a4c4c0a8300ae1620f6d5a2ea1a6b2e3e27f642260e132cf2ebf2a98ca03a46dd79b41568b2c53bf2889b4fdc5b6d454ddafaeb1f5abb2d3e010f39443fa04c98fa07640c08f77e2830d4053b2bc10346486216d7c5a6010f5c2c40665a67a06f3b8df2ce37cd2c596caf3750bfb7019091c29037edf66cae2cdfc273e567bda0e4b49398795c71b86a8dd3944953427e14d6e5e427ca0fde443e4505b9e2b9b8a06547bdf50b77d8ca8a059f8f96f10c89626b3fb4a99f944f596175a2f88de4d8a0f4558270c5aa5669fcc36424e4fc85758f41a17b9b1f0c3aa316488c5fcbc669a02778702c7a3769967dd42e639e24828de01ca11f47bd648fc4e0695b645fb469a009a0263ae6917980edc3950ea0e403ea36abed481a2da0f6d3de028af5b48029a004851336aece6f248c375b386aacf154b033caa45a2d35611ff11e0a53d8798480",
			"0xf8b1a09210595a62367dd0b3e8d43c941192fc5a916469c0a9b24517fb66d71ebd5a16808080808080a025ffe43610f734105480952603c8f0355e1b2ab509c66855ddd0cee3a332cc2880a0a6a4c159ee14e6e3a86df23d83bda0d84d1d061080c95e6cbbd0fe40024a3919808080a0eb0c333ce277240253bbf0fd22337c556342f58ba89503ac9cfdbc5de3facfff80a0f9589bb8289e455a36f1435e4612fbd1fee38851f0d8eae90da6f9122eaf51b280",
			"0xf8719d3e9a3e589d5f55bf39fc2428b31e3ec8ffcb7107dd2d1c5503fa1bdfb8b851f84f018b08e9358ffc243096c55045a0c1917a80cb25ccc50d0d1921525a44fb619b4601194ca726ae32312f08a799f8a06c029a231254fadb724d63be769f75eedd66362df034a3e663252b49d062a666"
		],
		"balance": "0x8e9358ffc243096c55045",
		"codeHash": "0x6c029a231254fadb724d63be769f75eedd66362df034a3e663252b49d062a666",
		"nonce": "0x1",
		"storageHash": "0xc1917a80cb25ccc50d0d1921525a44fb619b4601194ca726ae32312f08a799f8"
	}`
	var result eth.AccountResult
	err = json.Unmarshal([]byte(resultTestData), &result)
	assert.NoError(t, err)

	rpcCfg := &RPCConfig{
		ListenAddr: "localhost",
		ListenPort: 0,
	}
	rollupCfg := &rollup.Config{
		// ignore other rollup config info in this test
	}

	l2Client := &testutils.MockL2Client{}
	ref := eth.L2BlockRef{
		Hash:           header.Hash(),
		Number:         header.Number.Uint64(),
		ParentHash:     header.ParentHash,
		Time:           header.Time,
		L1Origin:       eth.BlockID{},
		SequenceNumber: 0,
	}
	output := &eth.OutputV0{
		StateRoot:                eth.Bytes32(header.Root),
		BlockHash:                ref.Hash,
		MessagePasserStorageRoot: eth.Bytes32(result.StorageHash),
	}
	l2Client.ExpectOutputV0AtBlock(common.HexToHash("0x8512bee03061475e4b069171f7b406097184f16b22c3f5c97c0abfc49591c524"), output, nil)

	drClient := &mockDriverClient{}
	safeReader := &mockSafeDBReader{}
	status := randomSyncStatus(rand.New(rand.NewSource(123)))
	drClient.ExpectBlockRefWithStatus(0xdcdc89, ref, status, nil)

	server, err := newRPCServer(rpcCfg, rollupCfg, l2Client, drClient, safeReader, log, "0.0", metrics.NoopMetrics)
	require.NoError(t, err)
	require.NoError(t, server.Start())
	defer func() {
		require.NoError(t, server.Stop(context.Background()))
	}()

	client, err := rpcclient.NewRPC(context.Background(), log, "http://"+server.Addr().String(), rpcclient.WithDialAttempts(3))
	require.NoError(t, err)

	var out *eth.OutputResponse
	err = client.CallContext(context.Background(), &out, "optimism_outputAtBlock", "0xdcdc89")
	require.NoError(t, err)

	require.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000000", out.Version.String())
	require.Equal(t, "0xc861dbdc5bf1d8bbbc0bca7cd876ab6a70748c50b2054a46e8f30e99002170ab", out.OutputRoot.String())
	require.Equal(t, "0xb46d4bcb0e471e1b8506031a1f34ebc6f200253cbaba56246dd2320e8e2c8f13", out.StateRoot.String())
	require.Equal(t, "0xc1917a80cb25ccc50d0d1921525a44fb619b4601194ca726ae32312f08a799f8", out.WithdrawalStorageRoot.String())
	require.Equal(t, *status, *out.Status)
	l2Client.Mock.AssertExpectations(t)
	drClient.Mock.AssertExpectations(t)
	safeReader.Mock.AssertExpectations(t)
}

func TestVersion(t *testing.T) {
	log := testlog.Logger(t, log.LevelError)
	l2Client := &testutils.MockL2Client{}
	drClient := &mockDriverClient{}
	safeReader := &mockSafeDBReader{}
	rpcCfg := &RPCConfig{
		ListenAddr: "localhost",
		ListenPort: 0,
	}
	rollupCfg := &rollup.Config{
		// ignore other rollup config info in this test
	}
	server, err := newRPCServer(rpcCfg, rollupCfg, l2Client, drClient, safeReader, log, "0.0", metrics.NoopMetrics)
	assert.NoError(t, err)
	assert.NoError(t, server.Start())
	defer func() {
		require.NoError(t, server.Stop(context.Background()))
	}()

	client, err := rpcclient.NewRPC(context.Background(), log, "http://"+server.Addr().String(), rpcclient.WithDialAttempts(3))
	assert.NoError(t, err)

	var out string
	err = client.CallContext(context.Background(), &out, "optimism_version")
	assert.NoError(t, err)
	assert.Equal(t, version.Version+"-"+version.Meta, out)
}

func randomSyncStatus(rng *rand.Rand) *eth.SyncStatus {
	return &eth.SyncStatus{
		CurrentL1:          testutils.RandomBlockRef(rng),
		CurrentL1Finalized: testutils.RandomBlockRef(rng),
		HeadL1:             testutils.RandomBlockRef(rng),
		SafeL1:             testutils.RandomBlockRef(rng),
		FinalizedL1:        testutils.RandomBlockRef(rng),
		UnsafeL2:           testutils.RandomL2BlockRef(rng),
		SafeL2:             testutils.RandomL2BlockRef(rng),
		FinalizedL2:        testutils.RandomL2BlockRef(rng),
		PendingSafeL2:      testutils.RandomL2BlockRef(rng),
	}
}

func TestSyncStatus(t *testing.T) {
	log := testlog.Logger(t, log.LevelError)
	l2Client := &testutils.MockL2Client{}
	drClient := &mockDriverClient{}
	safeReader := &mockSafeDBReader{}
	rng := rand.New(rand.NewSource(1234))
	status := randomSyncStatus(rng)
	drClient.On("SyncStatus").Return(status)

	rpcCfg := &RPCConfig{
		ListenAddr: "localhost",
		ListenPort: 0,
	}
	rollupCfg := &rollup.Config{
		// ignore other rollup config info in this test
	}
	server, err := newRPCServer(rpcCfg, rollupCfg, l2Client, drClient, safeReader, log, "0.0", metrics.NoopMetrics)
	assert.NoError(t, err)
	assert.NoError(t, server.Start())
	defer func() {
		require.NoError(t, server.Stop(context.Background()))
	}()

	client, err := rpcclient.NewRPC(context.Background(), log, "http://"+server.Addr().String(), rpcclient.WithDialAttempts(3))
	assert.NoError(t, err)

	var out *eth.SyncStatus
	err = client.CallContext(context.Background(), &out, "optimism_syncStatus")
	assert.NoError(t, err)
	assert.Equal(t, status, out)
}

func TestSafeHeadAtL1Block(t *testing.T) {
	log := testlog.Logger(t, log.LevelError)
	l2Client := &testutils.MockL2Client{}
	drClient := &mockDriverClient{}
	safeReader := &mockSafeDBReader{}
	l1BlockNum := uint64(5223)
	expectedL1 := eth.BlockID{
		Hash:   common.Hash{0xdd},
		Number: l1BlockNum - 2,
	}
	expectedSafeHead := eth.BlockID{
		Hash:   common.Hash{0xee},
		Number: 223,
	}
	expected := &eth.SafeHeadResponse{
		L1Block:  expectedL1,
		SafeHead: expectedSafeHead,
	}
	safeReader.ExpectSafeHeadAtL1(l1BlockNum, expectedL1, expectedSafeHead, nil)

	rpcCfg := &RPCConfig{
		ListenAddr: "localhost",
		ListenPort: 0,
	}
	rollupCfg := &rollup.Config{
		// ignore other rollup config info in this test
	}
	server, err := newRPCServer(rpcCfg, rollupCfg, l2Client, drClient, safeReader, log, "0.0", metrics.NoopMetrics)
	require.NoError(t, err)
	require.NoError(t, server.Start())
	defer func() {
		require.NoError(t, server.Stop(context.Background()))
	}()

	client, err := rpcclient.NewRPC(context.Background(), log, "http://"+server.Addr().String(), rpcclient.WithDialAttempts(3))
	require.NoError(t, err)

	var out *eth.SafeHeadResponse
	err = client.CallContext(context.Background(), &out, "optimism_safeHeadAtL1Block", hexutil.Uint64(l1BlockNum).String())
	require.NoError(t, err)
	require.Equal(t, expected, out)
	l2Client.Mock.AssertExpectations(t)
	drClient.Mock.AssertExpectations(t)
	safeReader.Mock.AssertExpectations(t)
}

type mockDriverClient struct {
	mock.Mock
}

func (c *mockDriverClient) ExpectBlockRefWithStatus(num uint64, ref eth.L2BlockRef, status *eth.SyncStatus, err error) {
	c.Mock.On("BlockRefWithStatus", num).Return(ref, status, &err)
}

func (c *mockDriverClient) BlockRefWithStatus(ctx context.Context, num uint64) (eth.L2BlockRef, *eth.SyncStatus, error) {
	m := c.Mock.MethodCalled("BlockRefWithStatus", num)
	return m[0].(eth.L2BlockRef), m[1].(*eth.SyncStatus), *m[2].(*error)
}

func (c *mockDriverClient) SyncStatus(ctx context.Context) (*eth.SyncStatus, error) {
	return c.Mock.MethodCalled("SyncStatus").Get(0).(*eth.SyncStatus), nil
}

func (c *mockDriverClient) ResetDerivationPipeline(ctx context.Context) error {
	return c.Mock.MethodCalled("ResetDerivationPipeline").Get(0).(error)
}

func (c *mockDriverClient) StartSequencer(ctx context.Context, blockHash common.Hash) error {
	return c.Mock.MethodCalled("StartSequencer").Get(0).(error)
}

func (c *mockDriverClient) StopSequencer(ctx context.Context) (common.Hash, error) {
	return c.Mock.MethodCalled("StopSequencer").Get(0).(common.Hash), nil
}

func (c *mockDriverClient) SequencerActive(ctx context.Context) (bool, error) {
	return c.Mock.MethodCalled("SequencerActive").Get(0).(bool), nil
}

func (c *mockDriverClient) OnUnsafeL2Payload(ctx context.Context, payload *eth.ExecutionPayloadEnvelope) error {
	return c.Mock.MethodCalled("OnUnsafeL2Payload").Get(0).(error)
}

func (c *mockDriverClient) OverrideLeader(ctx context.Context) error {
	return c.Mock.MethodCalled("OverrideLeader").Get(0).(error)
}

func (c *mockDriverClient) ConductorEnabled(ctx context.Context) (bool, error) {
	return c.Mock.MethodCalled("ConductorEnabled").Get(0).(bool), nil
}

type mockSafeDBReader struct {
	mock.Mock
}

func (m *mockSafeDBReader) SafeHeadAtL1(ctx context.Context, l1BlockNum uint64) (l1Hash eth.BlockID, l2Hash eth.BlockID, err error) {
	r := m.Mock.MethodCalled("SafeHeadAtL1", l1BlockNum)
	return r[0].(eth.BlockID), r[1].(eth.BlockID), *r[2].(*error)
}

func (m *mockSafeDBReader) ExpectSafeHeadAtL1(l1BlockNum uint64, l1 eth.BlockID, safeHead eth.BlockID, err error) {
	m.Mock.On("SafeHeadAtL1", l1BlockNum).Return(l1, safeHead, &err)
}
