package batcher

import (
	"fmt"
	"io"
	"testing"

	"github.com/ethereum-optimism/optimism/op-batcher/compressor"
	"github.com/ethereum-optimism/optimism/op-batcher/metrics"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

func singleFrameTxID(cid derive.ChannelID, fn uint16) txID {
	return txID{frameID{chID: cid, frameNumber: fn}}
}

func zeroFrameTxID(fn uint16) txID {
	return txID{frameID{frameNumber: fn}}
}

func newChannelWithChannelOut(log log.Logger, metr metrics.Metricer, cfg ChannelConfig, rollupCfg *rollup.Config, latestL1OriginBlockNum uint64) (*channel, error) {
	channelOut, err := NewChannelOut(cfg, rollupCfg)
	if err != nil {
		return nil, fmt.Errorf("creating channel out: %w", err)
	}
	return newChannel(log, metr, cfg, rollupCfg, latestL1OriginBlockNum, channelOut), nil
}

// TestChannelTimeout tests that the channel manager
// correctly identifies when a pending channel is timed out.
func TestChannelTimeout(t *testing.T) {
	// Create a new channel manager with a ChannelTimeout
	log := testlog.Logger(t, log.LevelCrit)
	m := NewChannelManager(log, metrics.NoopMetrics, ChannelConfig{
		ChannelTimeout: 100,
		CompressorConfig: compressor.Config{
			CompressionAlgo: derive.Zlib,
		},
	}, &rollup.Config{})
	m.Clear(eth.BlockID{})

	// Pending channel is nil so is cannot be timed out
	require.Nil(t, m.currentChannel)

	// Set the pending channel
	require.NoError(t, m.ensureChannelWithSpace(eth.BlockID{}))
	channel := m.currentChannel
	require.NotNil(t, channel)

	// add some pending txs, to be confirmed below
	channel.pendingTransactions[zeroFrameTxID(0).String()] = txData{}
	channel.pendingTransactions[zeroFrameTxID(1).String()] = txData{}
	channel.pendingTransactions[zeroFrameTxID(2).String()] = txData{}

	// There are no confirmed transactions so
	// the pending channel cannot be timed out
	timeout := channel.isTimedOut()
	require.False(t, timeout)

	// Manually confirm transactions
	channel.TxConfirmed(zeroFrameTxID(0).String(), eth.BlockID{Number: 0})
	channel.TxConfirmed(zeroFrameTxID(1).String(), eth.BlockID{Number: 99})

	// Since the ChannelTimeout is 100, the
	// pending channel should not be timed out
	timeout = channel.isTimedOut()
	require.False(t, timeout)

	// Add a confirmed transaction with a higher number
	// than the ChannelTimeout
	channel.TxConfirmed(zeroFrameTxID(2).String(), eth.BlockID{Number: 101})

	// Now the pending channel should be timed out
	timeout = channel.isTimedOut()
	require.True(t, timeout)
}

// TestChannelManager_NextTxData tests the nextTxData function.
func TestChannelManager_NextTxData(t *testing.T) {
	log := testlog.Logger(t, log.LevelCrit)
	m := NewChannelManager(log, metrics.NoopMetrics, ChannelConfig{CompressorConfig: compressor.Config{
		CompressionAlgo: derive.Zlib,
	}}, &rollup.Config{})
	m.Clear(eth.BlockID{})

	// Nil pending channel should return EOF
	returnedTxData, err := m.nextTxData(nil)
	require.ErrorIs(t, err, io.EOF)
	require.Equal(t, txData{}, returnedTxData)

	// Set the pending channel
	// The nextTxData function should still return io.EOF
	// since the current channel has no frames
	require.NoError(t, m.ensureChannelWithSpace(eth.BlockID{}))
	channel := m.currentChannel
	require.NotNil(t, channel)
	returnedTxData, err = m.nextTxData(channel)
	require.ErrorIs(t, err, io.EOF)
	require.Equal(t, txData{}, returnedTxData)

	// Manually push a frame into the pending channel
	channelID := channel.ID()
	frame := frameData{
		data: []byte{},
		id: frameID{
			chID:        channelID,
			frameNumber: uint16(0),
		},
	}
	channel.channelBuilder.frames = append(channel.channelBuilder.frames, frame)
	require.Equal(t, 1, channel.PendingFrames())

	// Now the nextTxData function should return the frame
	returnedTxData, err = m.nextTxData(channel)
	expectedTxData := singleFrameTxData(frame)
	expectedChannelID := expectedTxData.ID().String()
	require.NoError(t, err)
	require.Equal(t, expectedTxData, returnedTxData)
	require.Equal(t, 0, channel.PendingFrames())
	require.Equal(t, expectedTxData, channel.pendingTransactions[expectedChannelID])
}

func TestChannel_NextTxData_singleFrameTx(t *testing.T) {
	require := require.New(t)
	const n = 6
	lgr := testlog.Logger(t, log.LevelWarn)
	ch, err := newChannelWithChannelOut(lgr, metrics.NoopMetrics, ChannelConfig{
		UseBlobs:        false,
		TargetNumFrames: n,
		CompressorConfig: compressor.Config{
			CompressionAlgo: derive.Zlib,
		},
	}, &rollup.Config{}, latestL1BlockOrigin)
	require.NoError(err)
	chID := ch.ID()

	mockframes := makeMockFrameDatas(chID, n+1)
	// put multiple frames into channel, but less than target
	ch.channelBuilder.frames = mockframes[:n-1]

	requireTxData := func(i int) {
		require.True(ch.HasTxData(), "expected tx data %d", i)
		txdata := ch.NextTxData()
		require.Len(txdata.frames, 1)
		frame := txdata.frames[0]
		require.Len(frame.data, 1)
		require.EqualValues(i, frame.data[0])
		require.Equal(frameID{chID: chID, frameNumber: uint16(i)}, frame.id)
	}

	for i := 0; i < n-1; i++ {
		requireTxData(i)
	}
	require.False(ch.HasTxData())

	// put in last two
	ch.channelBuilder.frames = append(ch.channelBuilder.frames, mockframes[n-1:n+1]...)
	for i := n - 1; i < n+1; i++ {
		requireTxData(i)
	}
	require.False(ch.HasTxData())
}

func TestChannel_NextTxData_multiFrameTx(t *testing.T) {
	require := require.New(t)
	const n = eth.MaxBlobsPerBlobTx
	lgr := testlog.Logger(t, log.LevelWarn)
	ch, err := newChannelWithChannelOut(lgr, metrics.NoopMetrics, ChannelConfig{
		UseBlobs:        true,
		TargetNumFrames: n,
		CompressorConfig: compressor.Config{
			CompressionAlgo: derive.Zlib,
		},
	}, &rollup.Config{}, latestL1BlockOrigin)
	require.NoError(err)
	chID := ch.ID()

	mockframes := makeMockFrameDatas(chID, n+1)
	// put multiple frames into channel, but less than target
	ch.channelBuilder.frames = append(ch.channelBuilder.frames, mockframes[:n-1]...)
	require.False(ch.HasTxData())

	// put in last two
	ch.channelBuilder.frames = append(ch.channelBuilder.frames, mockframes[n-1:n+1]...)
	require.True(ch.HasTxData())
	txdata := ch.NextTxData()
	require.Len(txdata.frames, n)
	for i := 0; i < n; i++ {
		frame := txdata.frames[i]
		require.Len(frame.data, 1)
		require.EqualValues(i, frame.data[0])
		require.Equal(frameID{chID: chID, frameNumber: uint16(i)}, frame.id)
	}
	require.False(ch.HasTxData(), "no tx data expected with single pending frame")
}

func makeMockFrameDatas(id derive.ChannelID, n int) []frameData {
	fds := make([]frameData, 0, n)
	for i := 0; i < n; i++ {
		fds = append(fds, frameData{
			data: []byte{byte(i)},
			id: frameID{
				chID:        id,
				frameNumber: uint16(i),
			},
		})
	}
	return fds
}

// TestChannelTxConfirmed checks the [ChannelManager.TxConfirmed] function.
func TestChannelTxConfirmed(t *testing.T) {
	// Create a channel manager
	log := testlog.Logger(t, log.LevelCrit)
	m := NewChannelManager(log, metrics.NoopMetrics, ChannelConfig{
		// Need to set the channel timeout here so we don't clear pending
		// channels on confirmation. This would result in [TxConfirmed]
		// clearing confirmed transactions, and resetting the pendingChannels map
		ChannelTimeout: 10,
		CompressorConfig: compressor.Config{
			CompressionAlgo: derive.Zlib,
		},
	}, &rollup.Config{})
	m.Clear(eth.BlockID{})

	// Let's add a valid pending transaction to the channel manager
	// So we can demonstrate that TxConfirmed's correctness
	require.NoError(t, m.ensureChannelWithSpace(eth.BlockID{}))
	channelID := m.currentChannel.ID()
	frame := frameData{
		data: []byte{},
		id: frameID{
			chID:        channelID,
			frameNumber: uint16(0),
		},
	}
	m.currentChannel.channelBuilder.frames = append(m.currentChannel.channelBuilder.frames, frame)

	require.Equal(t, 1, m.currentChannel.PendingFrames())
	returnedTxData, err := m.nextTxData(m.currentChannel)
	expectedTxData := singleFrameTxData(frame)
	expectedChannelID := expectedTxData.ID()
	require.NoError(t, err)
	require.Equal(t, expectedTxData, returnedTxData)
	require.Equal(t, 0, m.currentChannel.PendingFrames())
	require.Equal(t, expectedTxData, m.currentChannel.pendingTransactions[expectedChannelID.String()])
	require.Len(t, m.currentChannel.pendingTransactions, 1)

	// An unknown pending transaction should not be marked as confirmed
	// and should not be removed from the pending transactions map
	actualChannelID := m.currentChannel.ID()
	unknownChannelID := derive.ChannelID([derive.ChannelIDLength]byte{0x69})
	require.NotEqual(t, actualChannelID, unknownChannelID)
	unknownTxID := singleFrameTxID(unknownChannelID, 0)
	blockID := eth.BlockID{Number: 0, Hash: common.Hash{0x69}}
	m.TxConfirmed(unknownTxID, blockID)
	require.Empty(t, m.currentChannel.confirmedTransactions)
	require.Len(t, m.currentChannel.pendingTransactions, 1)

	// Now let's mark the pending transaction as confirmed
	// and check that it is removed from the pending transactions map
	// and added to the confirmed transactions map
	m.TxConfirmed(expectedChannelID, blockID)
	require.Empty(t, m.currentChannel.pendingTransactions)
	require.Len(t, m.currentChannel.confirmedTransactions, 1)
	require.Equal(t, blockID, m.currentChannel.confirmedTransactions[expectedChannelID.String()])
}

// TestChannelTxFailed checks the [ChannelManager.TxFailed] function.
func TestChannelTxFailed(t *testing.T) {
	// Create a channel manager
	log := testlog.Logger(t, log.LevelCrit)
	m := NewChannelManager(log, metrics.NoopMetrics, ChannelConfig{CompressorConfig: compressor.Config{
		CompressionAlgo: derive.Zlib,
	}}, &rollup.Config{})
	m.Clear(eth.BlockID{})

	// Let's add a valid pending transaction to the channel
	// manager so we can demonstrate correctness
	require.NoError(t, m.ensureChannelWithSpace(eth.BlockID{}))
	channelID := m.currentChannel.ID()
	frame := frameData{
		data: []byte{},
		id: frameID{
			chID:        channelID,
			frameNumber: uint16(0),
		},
	}
	m.currentChannel.channelBuilder.frames = append(m.currentChannel.channelBuilder.frames, frame)
	require.Equal(t, 1, m.currentChannel.PendingFrames())
	returnedTxData, err := m.nextTxData(m.currentChannel)
	expectedTxData := singleFrameTxData(frame)
	expectedChannelID := expectedTxData.ID()
	require.NoError(t, err)
	require.Equal(t, expectedTxData, returnedTxData)
	require.Equal(t, 0, m.currentChannel.PendingFrames())
	require.Equal(t, expectedTxData, m.currentChannel.pendingTransactions[expectedChannelID.String()])
	require.Len(t, m.currentChannel.pendingTransactions, 1)

	// Trying to mark an unknown pending transaction as failed
	// shouldn't modify state
	m.TxFailed(zeroFrameTxID(0))
	require.Equal(t, 0, m.currentChannel.PendingFrames())
	require.Equal(t, expectedTxData, m.currentChannel.pendingTransactions[expectedChannelID.String()])

	// Now we still have a pending transaction
	// Let's mark it as failed
	m.TxFailed(expectedChannelID)
	require.Empty(t, m.currentChannel.pendingTransactions)
	// There should be a frame in the pending channel now
	require.Equal(t, 1, m.currentChannel.PendingFrames())
}
