package rollup

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum-optimism/optimism/op-service/eth"
)

func randConfig() *Config {
	rng := rand.New(rand.NewSource(1234))
	randHash := func() (out [32]byte) {
		rng.Read(out[:])
		return
	}
	randAddr := func() (out common.Address) { // we need generics...
		rng.Read(out[:])
		return
	}
	return &Config{
		Genesis: Genesis{
			L1:     eth.BlockID{Hash: randHash(), Number: 424242},
			L2:     eth.BlockID{Hash: randHash(), Number: 1337},
			L2Time: uint64(time.Now().Unix()),
			SystemConfig: eth.SystemConfig{
				BatcherAddr: randAddr(),
				Overhead:    randHash(),
				Scalar:      randHash(),
				GasLimit:    1234567,
			},
		},
		BlockTime:              2,
		MaxSequencerDrift:      100,
		SeqWindowSize:          2,
		ChannelTimeoutBedrock:  123,
		L1ChainID:              big.NewInt(900),
		L2ChainID:              big.NewInt(901),
		BatchInboxAddress:      randAddr(),
		DepositContractAddress: randAddr(),
		L1SystemConfigAddress:  randAddr(),
	}
}

func TestConfigJSON(t *testing.T) {
	config := randConfig()
	data, err := json.Marshal(config)
	assert.NoError(t, err)
	var roundTripped Config
	assert.NoError(t, json.Unmarshal(data, &roundTripped))
	assert.Equal(t, &roundTripped, config)
}

type mockL1Client struct {
	chainID *big.Int
	Hash    common.Hash
}

func (m *mockL1Client) ChainID(context.Context) (*big.Int, error) {
	return m.chainID, nil
}

func (m *mockL1Client) L1BlockRefByNumber(ctx context.Context, number uint64) (eth.L1BlockRef, error) {
	return eth.L1BlockRef{
		Hash:   m.Hash,
		Number: 100,
	}, nil
}

func TestValidateL1Config(t *testing.T) {
	config := randConfig()
	config.L1ChainID = big.NewInt(100)
	config.Genesis.L1.Number = 100
	config.Genesis.L1.Hash = [32]byte{0x01}
	mockClient := mockL1Client{chainID: big.NewInt(100), Hash: common.Hash{0x01}}
	err := config.ValidateL1Config(context.TODO(), &mockClient)
	assert.NoError(t, err)
}

func TestValidateL1ConfigInvalidChainIdFails(t *testing.T) {
	config := randConfig()
	config.L1ChainID = big.NewInt(101)
	config.Genesis.L1.Number = 100
	config.Genesis.L1.Hash = [32]byte{0x01}
	mockClient := mockL1Client{chainID: big.NewInt(100), Hash: common.Hash{0x01}}
	err := config.ValidateL1Config(context.TODO(), &mockClient)
	assert.Error(t, err)
	config.L1ChainID = big.NewInt(99)
	err = config.ValidateL1Config(context.TODO(), &mockClient)
	assert.Error(t, err)
}

func TestValidateL1ConfigInvalidGenesisHashFails(t *testing.T) {
	config := randConfig()
	config.L1ChainID = big.NewInt(100)
	config.Genesis.L1.Number = 100
	config.Genesis.L1.Hash = [32]byte{0x00}
	mockClient := mockL1Client{chainID: big.NewInt(100), Hash: common.Hash{0x01}}
	err := config.ValidateL1Config(context.TODO(), &mockClient)
	assert.Error(t, err)
	config.Genesis.L1.Hash = [32]byte{0x02}
	err = config.ValidateL1Config(context.TODO(), &mockClient)
	assert.Error(t, err)
}

func TestCheckL1ChainID(t *testing.T) {
	config := randConfig()
	config.L1ChainID = big.NewInt(100)
	err := config.CheckL1ChainID(context.TODO(), &mockL1Client{chainID: big.NewInt(100)})
	assert.NoError(t, err)
	err = config.CheckL1ChainID(context.TODO(), &mockL1Client{chainID: big.NewInt(101)})
	assert.Error(t, err)
	err = config.CheckL1ChainID(context.TODO(), &mockL1Client{chainID: big.NewInt(99)})
	assert.Error(t, err)
}

func TestCheckL1BlockRefByNumber(t *testing.T) {
	config := randConfig()
	config.Genesis.L1.Number = 100
	config.Genesis.L1.Hash = [32]byte{0x01}
	mockClient := mockL1Client{chainID: big.NewInt(100), Hash: common.Hash{0x01}}
	err := config.CheckL1GenesisBlockHash(context.TODO(), &mockClient)
	assert.NoError(t, err)
	mockClient.Hash = common.Hash{0x02}
	err = config.CheckL1GenesisBlockHash(context.TODO(), &mockClient)
	assert.Error(t, err)
	mockClient.Hash = common.Hash{0x00}
	err = config.CheckL1GenesisBlockHash(context.TODO(), &mockClient)
	assert.Error(t, err)
}

// TestRandomConfigDescription tests that the description works for different variations of a random rollup config.
func TestRandomConfigDescription(t *testing.T) {
	t.Run("named L2", func(t *testing.T) {
		config := randConfig()
		out := config.Description(map[string]string{config.L2ChainID.String(): "foobar chain"})
		require.Contains(t, out, "foobar chain")
	})
	t.Run("named L1", func(t *testing.T) {
		config := randConfig()
		config.L1ChainID = big.NewInt(11155111)
		out := config.Description(map[string]string{config.L2ChainID.String(): "foobar chain"})
		require.Contains(t, out, "sepolia")
	})
	t.Run("unnamed", func(t *testing.T) {
		config := randConfig()
		out := config.Description(nil)
		require.Contains(t, out, "(unknown L1)")
		require.Contains(t, out, "(unknown L2)")
	})
	t.Run("regolith unset", func(t *testing.T) {
		config := randConfig()
		config.RegolithTime = nil
		out := config.Description(nil)
		require.Contains(t, out, "Regolith: (not configured)")
	})
	t.Run("regolith genesis", func(t *testing.T) {
		config := randConfig()
		config.RegolithTime = new(uint64)
		out := config.Description(nil)
		require.Contains(t, out, "Regolith: @ genesis")
	})
	t.Run("regolith date", func(t *testing.T) {
		config := randConfig()
		x := uint64(1677119335)
		config.RegolithTime = &x
		out := config.Description(nil)
		// Don't check human-readable part of the date, it's timezone-dependent.
		// Don't make this test fail only in Australia :')
		require.Contains(t, out, fmt.Sprintf("Regolith: @ %d ~ ", x))
	})
}

// TestActivations tests the activation condition of the various upgrades.
func TestActivations(t *testing.T) {
	for _, test := range []struct {
		name           string
		setUpgradeTime func(t *uint64, c *Config)
		checkEnabled   func(t uint64, c *Config) bool
	}{
		{
			name: "Regolith",
			setUpgradeTime: func(t *uint64, c *Config) {
				c.RegolithTime = t
			},
			checkEnabled: func(t uint64, c *Config) bool {
				return c.IsRegolith(t)
			},
		},
		{
			name: "Canyon",
			setUpgradeTime: func(t *uint64, c *Config) {
				c.CanyonTime = t
			},
			checkEnabled: func(t uint64, c *Config) bool {
				return c.IsCanyon(t)
			},
		},
		{
			name: "Delta",
			setUpgradeTime: func(t *uint64, c *Config) {
				c.DeltaTime = t
			},
			checkEnabled: func(t uint64, c *Config) bool {
				return c.IsDelta(t)
			},
		},
		{
			name: "Ecotone",
			setUpgradeTime: func(t *uint64, c *Config) {
				c.EcotoneTime = t
			},
			checkEnabled: func(t uint64, c *Config) bool {
				return c.IsEcotone(t)
			},
		},
		{
			name: "Fjord",
			setUpgradeTime: func(t *uint64, c *Config) {
				c.FjordTime = t
			},
			checkEnabled: func(t uint64, c *Config) bool {
				return c.IsFjord(t)
			},
		},
		{
			name: "Granite",
			setUpgradeTime: func(t *uint64, c *Config) {
				c.GraniteTime = t
			},
			checkEnabled: func(t uint64, c *Config) bool {
				return c.IsGranite(t)
			},
		},
		{
			name: "Holocene",
			setUpgradeTime: func(t *uint64, c *Config) {
				c.HoloceneTime = t
			},
			checkEnabled: func(t uint64, c *Config) bool {
				return c.IsHolocene(t)
			},
		},
		{
			name: "Interop",
			setUpgradeTime: func(t *uint64, c *Config) {
				c.InteropTime = t
			},
			checkEnabled: func(t uint64, c *Config) bool {
				return c.IsInterop(t)
			},
		},
	} {
		tt := test
		t.Run(fmt.Sprintf("TestActivations_%s", tt.name), func(t *testing.T) {
			config := randConfig()
			test.setUpgradeTime(nil, config)
			require.False(t, tt.checkEnabled(0, config), "false if nil time, even if checking 0")
			require.False(t, tt.checkEnabled(123456, config), "false if nil time")

			test.setUpgradeTime(new(uint64), config)
			require.True(t, tt.checkEnabled(0, config), "true at zero")
			require.True(t, tt.checkEnabled(123456, config), "true for any")

			x := uint64(123)
			test.setUpgradeTime(&x, config)
			require.False(t, tt.checkEnabled(0, config))
			require.False(t, tt.checkEnabled(122, config))
			require.True(t, tt.checkEnabled(123, config))
			require.True(t, tt.checkEnabled(124, config))
		})
	}
}

type mockL2Client struct {
	chainID *big.Int
	Hash    common.Hash
}

func (m *mockL2Client) ChainID(context.Context) (*big.Int, error) {
	return m.chainID, nil
}

func (m *mockL2Client) L2BlockRefByNumber(ctx context.Context, number uint64) (eth.L2BlockRef, error) {
	return eth.L2BlockRef{
		Hash:   m.Hash,
		Number: 100,
	}, nil
}

func TestValidateL2Config(t *testing.T) {
	config := randConfig()
	config.L2ChainID = big.NewInt(100)
	config.Genesis.L2.Number = 100
	config.Genesis.L2.Hash = [32]byte{0x01}
	mockClient := mockL2Client{chainID: big.NewInt(100), Hash: common.Hash{0x01}}
	err := config.ValidateL2Config(context.TODO(), &mockClient, false)
	assert.NoError(t, err)
}

func TestValidateL2ConfigInvalidChainIdFails(t *testing.T) {
	config := randConfig()
	config.L2ChainID = big.NewInt(101)
	config.Genesis.L2.Number = 100
	config.Genesis.L2.Hash = [32]byte{0x01}
	mockClient := mockL2Client{chainID: big.NewInt(100), Hash: common.Hash{0x01}}
	err := config.ValidateL2Config(context.TODO(), &mockClient, false)
	assert.Error(t, err)
	config.L2ChainID = big.NewInt(99)
	err = config.ValidateL2Config(context.TODO(), &mockClient, false)
	assert.Error(t, err)
}

func TestValidateL2ConfigInvalidGenesisHashFails(t *testing.T) {
	config := randConfig()
	config.L2ChainID = big.NewInt(100)
	config.Genesis.L2.Number = 100
	config.Genesis.L2.Hash = [32]byte{0x00}
	mockClient := mockL2Client{chainID: big.NewInt(100), Hash: common.Hash{0x01}}
	err := config.ValidateL2Config(context.TODO(), &mockClient, false)
	assert.Error(t, err)
	config.Genesis.L2.Hash = [32]byte{0x02}
	err = config.ValidateL2Config(context.TODO(), &mockClient, false)
	assert.Error(t, err)
}

func TestValidateL2ConfigInvalidGenesisHashSkippedWhenRequested(t *testing.T) {
	config := randConfig()
	config.L2ChainID = big.NewInt(100)
	config.Genesis.L2.Number = 100
	config.Genesis.L2.Hash = [32]byte{0x00}
	mockClient := mockL2Client{chainID: big.NewInt(100), Hash: common.Hash{0x01}}
	err := config.ValidateL2Config(context.TODO(), &mockClient, true)
	assert.NoError(t, err)
	config.Genesis.L2.Hash = [32]byte{0x02}
	err = config.ValidateL2Config(context.TODO(), &mockClient, true)
	assert.NoError(t, err)
}

func TestCheckL2ChainID(t *testing.T) {
	config := randConfig()
	config.L2ChainID = big.NewInt(100)
	err := config.CheckL2ChainID(context.TODO(), &mockL2Client{chainID: big.NewInt(100)})
	assert.NoError(t, err)
	err = config.CheckL2ChainID(context.TODO(), &mockL2Client{chainID: big.NewInt(101)})
	assert.Error(t, err)
	err = config.CheckL2ChainID(context.TODO(), &mockL2Client{chainID: big.NewInt(99)})
	assert.Error(t, err)
}

func TestCheckL2BlockRefByNumber(t *testing.T) {
	config := randConfig()
	config.Genesis.L2.Number = 100
	config.Genesis.L2.Hash = [32]byte{0x01}
	mockClient := mockL2Client{chainID: big.NewInt(100), Hash: common.Hash{0x01}}
	err := config.CheckL2GenesisBlockHash(context.TODO(), &mockClient)
	assert.NoError(t, err)
	mockClient.Hash = common.Hash{0x02}
	err = config.CheckL2GenesisBlockHash(context.TODO(), &mockClient)
	assert.Error(t, err)
	mockClient.Hash = common.Hash{0x00}
	err = config.CheckL2GenesisBlockHash(context.TODO(), &mockClient)
	assert.Error(t, err)
}

func TestConfig_Check(t *testing.T) {
	tests := []struct {
		name        string
		modifier    func(cfg *Config)
		expectedErr error
	}{
		{
			name:        "BlockTimeZero",
			modifier:    func(cfg *Config) { cfg.BlockTime = 0 },
			expectedErr: ErrBlockTimeZero,
		},
		{
			name:        "ChannelTimeoutBedrockZero",
			modifier:    func(cfg *Config) { cfg.ChannelTimeoutBedrock = 0 },
			expectedErr: ErrMissingChannelTimeout,
		},
		{
			name:        "SeqWindowSizeZero",
			modifier:    func(cfg *Config) { cfg.SeqWindowSize = 0 },
			expectedErr: ErrInvalidSeqWindowSize,
		},
		{
			name:        "SeqWindowSizeOne",
			modifier:    func(cfg *Config) { cfg.SeqWindowSize = 1 },
			expectedErr: ErrInvalidSeqWindowSize,
		},
		{
			name:        "NoL1Genesis",
			modifier:    func(cfg *Config) { cfg.Genesis.L1.Hash = common.Hash{} },
			expectedErr: ErrMissingGenesisL1Hash,
		},
		{
			name:        "NoL2Genesis",
			modifier:    func(cfg *Config) { cfg.Genesis.L2.Hash = common.Hash{} },
			expectedErr: ErrMissingGenesisL2Hash,
		},
		{
			name:        "GenesisHashesEqual",
			modifier:    func(cfg *Config) { cfg.Genesis.L2.Hash = cfg.Genesis.L1.Hash },
			expectedErr: ErrGenesisHashesSame,
		},
		{
			name:        "GenesisL2TimeZero",
			modifier:    func(cfg *Config) { cfg.Genesis.L2Time = 0 },
			expectedErr: ErrMissingGenesisL2Time,
		},
		{
			name:        "NoBatcherAddr",
			modifier:    func(cfg *Config) { cfg.Genesis.SystemConfig.BatcherAddr = common.Address{} },
			expectedErr: ErrMissingBatcherAddr,
		},
		{
			name:        "NoScalar",
			modifier:    func(cfg *Config) { cfg.Genesis.SystemConfig.Scalar = eth.Bytes32{} },
			expectedErr: ErrMissingScalar,
		},
		{
			name:        "NoGasLimit",
			modifier:    func(cfg *Config) { cfg.Genesis.SystemConfig.GasLimit = 0 },
			expectedErr: ErrMissingGasLimit,
		},
		{
			name:        "NoBatchInboxAddress",
			modifier:    func(cfg *Config) { cfg.BatchInboxAddress = common.Address{} },
			expectedErr: ErrMissingBatchInboxAddress,
		},
		{
			name:        "NoDepositContractAddress",
			modifier:    func(cfg *Config) { cfg.DepositContractAddress = common.Address{} },
			expectedErr: ErrMissingDepositContractAddress,
		},
		{
			name:        "NoL1ChainId",
			modifier:    func(cfg *Config) { cfg.L1ChainID = nil },
			expectedErr: ErrMissingL1ChainID,
		},
		{
			name:        "NoL2ChainId",
			modifier:    func(cfg *Config) { cfg.L2ChainID = nil },
			expectedErr: ErrMissingL2ChainID,
		},
		{
			name:        "ChainIDsEqual",
			modifier:    func(cfg *Config) { cfg.L2ChainID = cfg.L1ChainID },
			expectedErr: ErrChainIDsSame,
		},
		{
			name:        "L1ChainIdNegative",
			modifier:    func(cfg *Config) { cfg.L1ChainID = big.NewInt(-1) },
			expectedErr: ErrL1ChainIDNotPositive,
		},
		{
			name:        "L1ChainIdZero",
			modifier:    func(cfg *Config) { cfg.L1ChainID = big.NewInt(0) },
			expectedErr: ErrL1ChainIDNotPositive,
		},
		{
			name:        "L2ChainIdNegative",
			modifier:    func(cfg *Config) { cfg.L2ChainID = big.NewInt(-1) },
			expectedErr: ErrL2ChainIDNotPositive,
		},
		{
			name:        "L2ChainIdZero",
			modifier:    func(cfg *Config) { cfg.L2ChainID = big.NewInt(0) },
			expectedErr: ErrL2ChainIDNotPositive,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := randConfig()
			test.modifier(cfg)
			err := cfg.Check()
			assert.ErrorIs(t, err, test.expectedErr)
		})
	}

	forkTests := []struct {
		name        string
		modifier    func(cfg *Config)
		expectedErr error
	}{
		{
			name: "PriorForkMissing",
			modifier: func(cfg *Config) {
				ecotoneTime := uint64(1)
				cfg.EcotoneTime = &ecotoneTime
			},
			expectedErr: fmt.Errorf("fork ecotone set (to 1), but prior fork delta missing"),
		},
		{
			name: "PriorForkHasHigherOffset",
			modifier: func(cfg *Config) {
				regolithTime := uint64(2)
				canyonTime := uint64(1)
				cfg.RegolithTime = &regolithTime
				cfg.CanyonTime = &canyonTime
			},
			expectedErr: fmt.Errorf("fork canyon set to 1, but prior fork regolith has higher offset 2"),
		},
		{
			name: "PriorForkOK",
			modifier: func(cfg *Config) {
				regolithTime := uint64(1)
				canyonTime := uint64(2)
				deltaTime := uint64(3)
				ecotoneTime := uint64(4)
				cfg.RegolithTime = &regolithTime
				cfg.CanyonTime = &canyonTime
				cfg.DeltaTime = &deltaTime
				cfg.EcotoneTime = &ecotoneTime
			},
			expectedErr: nil,
		},
	}

	for _, test := range forkTests {
		t.Run(test.name, func(t *testing.T) {
			cfg := randConfig()
			test.modifier(cfg)
			err := cfg.Check()
			assert.Equal(t, err, test.expectedErr)
		})
	}
}

func TestTimestampForBlock(t *testing.T) {
	config := randConfig()

	tests := []struct {
		name              string
		genesisTime       uint64
		genesisBlock      uint64
		blockTime         uint64
		blockNum          uint64
		expectedBlockTime uint64
	}{
		{
			name:              "FirstBlock",
			genesisTime:       100,
			genesisBlock:      0,
			blockTime:         2,
			blockNum:          0,
			expectedBlockTime: 100,
		},
		{
			name:              "SecondBlock",
			genesisTime:       100,
			genesisBlock:      0,
			blockTime:         2,
			blockNum:          1,
			expectedBlockTime: 102,
		},
		{
			name:              "NBlock",
			genesisTime:       100,
			genesisBlock:      0,
			blockTime:         2,
			blockNum:          25,
			expectedBlockTime: 150,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(fmt.Sprintf("TestTimestampForBlock_%s", test.name), func(t *testing.T) {
			config.Genesis.L2Time = test.genesisTime
			config.Genesis.L2.Number = test.genesisBlock
			config.BlockTime = test.blockTime

			timestamp := config.TimestampForBlock(test.blockNum)
			assert.Equal(t, timestamp, test.expectedBlockTime)
		})
	}
}

func TestForkchoiceUpdatedVersion(t *testing.T) {
	config := randConfig()
	tests := []struct {
		name           string
		canyonTime     uint64
		ecotoneTime    uint64
		attrs          *eth.PayloadAttributes
		expectedMethod eth.EngineAPIMethod
	}{
		{
			name:           "NoAttrs",
			canyonTime:     10,
			ecotoneTime:    20,
			attrs:          nil,
			expectedMethod: eth.FCUV3,
		},
		{
			name:           "BeforeCanyon",
			canyonTime:     10,
			ecotoneTime:    20,
			attrs:          &eth.PayloadAttributes{Timestamp: 5},
			expectedMethod: eth.FCUV1,
		},
		{
			name:           "Canyon",
			canyonTime:     10,
			ecotoneTime:    20,
			attrs:          &eth.PayloadAttributes{Timestamp: 15},
			expectedMethod: eth.FCUV2,
		},
		{
			name:           "Ecotone",
			canyonTime:     10,
			ecotoneTime:    20,
			attrs:          &eth.PayloadAttributes{Timestamp: 25},
			expectedMethod: eth.FCUV3,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(fmt.Sprintf("TestForkchoiceUpdatedVersion_%s", test.name), func(t *testing.T) {
			config.CanyonTime = &test.canyonTime
			config.EcotoneTime = &test.ecotoneTime
			assert.Equal(t, config.ForkchoiceUpdatedVersion(test.attrs), test.expectedMethod)
		})
	}
}

func TestNewPayloadVersion(t *testing.T) {
	config := randConfig()
	canyonTime := uint64(0)
	config.CanyonTime = &canyonTime
	tests := []struct {
		name           string
		ecotoneTime    uint64
		payloadTime    uint64
		expectedMethod eth.EngineAPIMethod
	}{
		{
			name:           "BeforeEcotone",
			ecotoneTime:    10,
			payloadTime:    5,
			expectedMethod: eth.NewPayloadV2,
		},
		{
			name:           "Ecotone",
			ecotoneTime:    10,
			payloadTime:    15,
			expectedMethod: eth.NewPayloadV3,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(fmt.Sprintf("TestNewPayloadVersion_%s", test.name), func(t *testing.T) {
			config.EcotoneTime = &test.ecotoneTime
			assert.Equal(t, config.NewPayloadVersion(test.payloadTime), test.expectedMethod)
		})
	}
}

func TestGetPayloadVersion(t *testing.T) {
	config := randConfig()
	canyonTime := uint64(0)
	config.CanyonTime = &canyonTime
	tests := []struct {
		name           string
		ecotoneTime    uint64
		payloadTime    uint64
		expectedMethod eth.EngineAPIMethod
	}{
		{
			name:           "BeforeEcotone",
			ecotoneTime:    10,
			payloadTime:    5,
			expectedMethod: eth.GetPayloadV2,
		},
		{
			name:           "Ecotone",
			ecotoneTime:    10,
			payloadTime:    15,
			expectedMethod: eth.GetPayloadV3,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(fmt.Sprintf("TestGetPayloadVersion_%s", test.name), func(t *testing.T) {
			config.EcotoneTime = &test.ecotoneTime
			assert.Equal(t, config.GetPayloadVersion(test.payloadTime), test.expectedMethod)
		})
	}
}

func TestConfig_IsActivationBlock(t *testing.T) {
	ts := uint64(42)
	// TODO(12490): Currently only supports Holocene. Will be modularized in a follow-up.
	for _, fork := range []ForkName{Holocene} {
		cfg := &Config{
			HoloceneTime: &ts,
		}
		require.Equal(t, fork, cfg.IsActivationBlock(0, ts))
		require.Equal(t, fork, cfg.IsActivationBlock(0, ts+64))
		require.Equal(t, fork, cfg.IsActivationBlock(ts-1, ts))
		require.Equal(t, fork, cfg.IsActivationBlock(ts-1, ts+1))
		require.Zero(t, cfg.IsActivationBlock(0, ts-1))
		require.Zero(t, cfg.IsActivationBlock(ts, ts+1))
	}
}
