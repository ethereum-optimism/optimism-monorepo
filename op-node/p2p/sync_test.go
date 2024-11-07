package p2p

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-node/metrics"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

type mockPayloadFn func(n uint64) (*eth.ExecutionPayloadEnvelope, error)

func (fn mockPayloadFn) PayloadByNumber(_ context.Context, number uint64) (*eth.ExecutionPayloadEnvelope, error) {
	return fn(number)
}

var _ L2Chain = mockPayloadFn(nil)

type syncTestData struct {
	sync.RWMutex
	payloads map[uint64]*eth.ExecutionPayloadEnvelope
}

func (s *syncTestData) getPayload(i uint64) (payload *eth.ExecutionPayloadEnvelope, ok bool) {
	s.RLock()
	defer s.RUnlock()
	payload, ok = s.payloads[i]
	return payload, ok
}

func (s *syncTestData) deletePayload(i uint64) {
	s.Lock()
	defer s.Unlock()
	delete(s.payloads, i)
}

func (s *syncTestData) addPayload(payload *eth.ExecutionPayloadEnvelope) {
	s.Lock()
	defer s.Unlock()
	s.payloads[uint64(payload.ExecutionPayload.BlockNumber)] = payload
}

func (s *syncTestData) getBlockRef(i uint64) eth.L2BlockRef {
	s.RLock()
	defer s.RUnlock()
	return eth.L2BlockRef{
		Hash:       s.payloads[i].ExecutionPayload.BlockHash,
		Number:     uint64(s.payloads[i].ExecutionPayload.BlockNumber),
		ParentHash: s.payloads[i].ExecutionPayload.ParentHash,
		Time:       uint64(s.payloads[i].ExecutionPayload.Timestamp),
	}
}

func setupSyncTestData(length uint64) (*rollup.Config, *syncTestData) {
	// minimal rollup config to build mock blocks & verify their time.
	cfg := &rollup.Config{
		Genesis: rollup.Genesis{
			L1:     eth.BlockID{Hash: common.Hash{0xaa}},
			L2:     eth.BlockID{Hash: common.Hash{0xbb}},
			L2Time: 9000,
		},
		BlockTime: 2,
		L2ChainID: big.NewInt(1234),
	}

	ecotoneBlock := length / 2
	ecotoneTime := cfg.Genesis.L2Time + ecotoneBlock*cfg.BlockTime
	cfg.EcotoneTime = &ecotoneTime

	// create some simple fake test blocks
	payloads := make(map[uint64]*eth.ExecutionPayloadEnvelope)
	payloads[0] = &eth.ExecutionPayloadEnvelope{
		ExecutionPayload: &eth.ExecutionPayload{
			Timestamp: eth.Uint64Quantity(cfg.Genesis.L2Time),
		},
	}

	payloads[0].ExecutionPayload.BlockHash, _ = payloads[0].CheckBlockHash()
	for i := uint64(1); i <= length; i++ {
		timestamp := cfg.Genesis.L2Time + i*cfg.BlockTime
		payload := &eth.ExecutionPayloadEnvelope{
			ExecutionPayload: &eth.ExecutionPayload{
				ParentHash:  payloads[i-1].ExecutionPayload.BlockHash,
				BlockNumber: eth.Uint64Quantity(i),
				Timestamp:   eth.Uint64Quantity(timestamp),
			},
		}

		if cfg.IsEcotone(timestamp) {
			hash := common.BigToHash(big.NewInt(int64(i)))
			payload.ParentBeaconBlockRoot = &hash

			zero := eth.Uint64Quantity(0)
			payload.ExecutionPayload.ExcessBlobGas = &zero
			payload.ExecutionPayload.BlobGasUsed = &zero

			w := types.Withdrawals{}
			payload.ExecutionPayload.Withdrawals = &w
		}

		payload.ExecutionPayload.BlockHash, _ = payload.CheckBlockHash()
		payloads[i] = payload
	}

	return cfg, &syncTestData{payloads: payloads}
}

func TestSinglePeerSync(t *testing.T) {
	t.Parallel() // Takes a while, but can run in parallel

	log := testlog.Logger(t, log.LevelError)

	cfg, payloads := setupSyncTestData(25)

	// Serving payloads: just load them from the map, if they exist
	servePayload := mockPayloadFn(func(n uint64) (*eth.ExecutionPayloadEnvelope, error) {
		p, ok := payloads.getPayload(n)
		if !ok {
			return nil, ethereum.NotFound
		}
		return p, nil
	})

	// collect received payloads in a buffered channel, so we can verify we get everything
	received := make(chan *eth.ExecutionPayloadEnvelope, 100)
	receivePayload := receivePayloadFn(func(ctx context.Context, from peer.ID, payload *eth.ExecutionPayloadEnvelope) error {
		received <- payload
		return nil
	})

	// Setup 2 minimal test hosts to attach the sync protocol to
	mnet, err := mocknet.FullMeshConnected(2)
	require.NoError(t, err, "failed to setup mocknet")
	defer mnet.Close()
	hosts := mnet.Hosts()
	hostA, hostB := hosts[0], hosts[1]
	require.Equal(t, hostA.Network().Connectedness(hostB.ID()), network.Connected)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup host A as the server
	srv := NewReqRespServer(cfg, servePayload, metrics.NoopMetrics)
	payloadByNumber := MakeStreamHandler(ctx, log.New("role", "server"), srv.HandleSyncRequest)
	hostA.SetStreamHandler(PayloadByNumberProtocolID(cfg.L2ChainID), payloadByNumber)

	// Setup host B as the client
	cl := NewSyncClient(log.New("role", "client"), cfg, hostB, receivePayload, metrics.NoopMetrics, &NoopApplicationScorer{})

	// Setup host B (client) to sync from its peer Host A (server)
	cl.AddPeer(hostA.ID())
	cl.Start()
	defer cl.Close()

	// request to start syncing between 10 and 20
	_, err = cl.RequestL2Range(ctx, payloads.getBlockRef(10), payloads.getBlockRef(20))
	require.NoError(t, err)

	// and wait for the sync results to come in (in reverse order)
	for i := uint64(19); i > 10; i-- {
		p := <-received
		require.Equal(t, uint64(p.ExecutionPayload.BlockNumber), i, "expecting payloads in order")
		exp, ok := payloads.getPayload(uint64(p.ExecutionPayload.BlockNumber))
		require.True(t, ok, "expecting known payload")
		require.Equal(t, exp.ExecutionPayload.BlockHash, p.ExecutionPayload.BlockHash, "expecting the correct payload")

		require.Equal(t, exp.ParentBeaconBlockRoot, p.ParentBeaconBlockRoot)
		if cfg.IsEcotone(uint64(p.ExecutionPayload.Timestamp)) {
			require.NotNil(t, p.ParentBeaconBlockRoot)
		} else {
			require.Nil(t, p.ParentBeaconBlockRoot)
		}
	}
}

func TestMultiPeerSync(t *testing.T) {
	t.Parallel() // Takes a while, but can run in parallel

	log := testlog.Logger(t, log.LevelDebug)

	cfg, payloads := setupSyncTestData(100)

	// Buffered channel of all blocks requested from any client.
	requested := make(chan uint64, 100)

	setupPeer := func(ctx context.Context, h host.Host) (*SyncClient, chan *eth.ExecutionPayloadEnvelope) {
		// Serving payloads: just load them from the map, if they exist
		servePayload := mockPayloadFn(func(n uint64) (*eth.ExecutionPayloadEnvelope, error) {
			requested <- n
			p, ok := payloads.getPayload(n)
			if !ok {
				return nil, ethereum.NotFound
			}
			return p, nil
		})

		// collect received payloads in a buffered channel, so we can verify we get everything
		received := make(chan *eth.ExecutionPayloadEnvelope, 100)
		receivePayload := receivePayloadFn(func(ctx context.Context, from peer.ID, payload *eth.ExecutionPayloadEnvelope) error {
			received <- payload
			return nil
		})

		// Setup as server
		srv := NewReqRespServer(cfg, servePayload, metrics.NoopMetrics)
		payloadByNumber := MakeStreamHandler(ctx, log.New("serve", "payloads_by_number"), srv.HandleSyncRequest)
		h.SetStreamHandler(PayloadByNumberProtocolID(cfg.L2ChainID), payloadByNumber)

		cl := NewSyncClient(log.New("role", "client"), cfg, h, receivePayload, metrics.NoopMetrics, &NoopApplicationScorer{})
		return cl, received
	}

	// Setup 3 minimal test hosts to attach the sync protocol to
	mnet, err := mocknet.FullMeshConnected(3)
	require.NoError(t, err, "failed to setup mocknet")
	defer mnet.Close()
	hosts := mnet.Hosts()
	hostA, hostB, hostC := hosts[0], hosts[1], hosts[2]
	require.Equal(t, hostA.Network().Connectedness(hostB.ID()), network.Connected)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clA, recvA := setupPeer(ctx, hostA)
	clB, recvB := setupPeer(ctx, hostB)
	clC, _ := setupPeer(ctx, hostC)

	// Make them all sync from each other
	clA.AddPeer(hostB.ID())
	clA.AddPeer(hostC.ID())
	clA.Start()
	defer clA.Close()
	clB.AddPeer(hostA.ID())
	clB.AddPeer(hostC.ID())
	clB.Start()
	defer clB.Close()
	clC.AddPeer(hostA.ID())
	clC.AddPeer(hostB.ID())
	clC.Start()
	defer clC.Close()

	// request to start syncing between 10 and 90
	_, err = clA.RequestL2Range(ctx, payloads.getBlockRef(10), payloads.getBlockRef(90))
	require.NoError(t, err)

	// With such large range to request we are going to hit the rate-limits of B and C,
	// but that means we'll balance the work between the peers.
	for i := uint64(89); i > 10; i-- { // wait for all payloads
		e := <-recvA
		p := e.ExecutionPayload
		exp, ok := payloads.getPayload(uint64(p.BlockNumber))
		require.True(t, ok, "expecting known payload")
		require.Equal(t, exp.ExecutionPayload.BlockHash, p.BlockHash, "expecting the correct payload")
	}

	// now see if B can sync a range, and fill the gap with a re-request
	bl25, _ := payloads.getPayload(25) // temporarily remove it from the available payloads. This will create a gap
	payloads.deletePayload(25)
	rangeReqId, err := clB.RequestL2Range(ctx, payloads.getBlockRef(20), payloads.getBlockRef(30))

	require.NoError(t, err)
	require.True(t, clB.activeRangeRequests.get(rangeReqId), "expecting range request to be active")

	for i := uint64(29); i > 25; i-- {
		p := <-recvB
		exp, ok := payloads.getPayload(uint64(p.ExecutionPayload.BlockNumber))
		require.True(t, ok, "expecting known payload")
		require.Equal(t, exp.ExecutionPayload.BlockHash, p.ExecutionPayload.BlockHash, "expecting the correct payload")
	}

	// Wait for the request for block 25 to be made
	ctx, cancelFunc := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelFunc()
	requestMade := false
	for requestMade != true {
		select {
		case blockNum := <-requested:
			if blockNum == 25 {
				requestMade = true
			}
		case <-ctx.Done():
			t.Fatal("Did not request block 25 in a reasonable time")
		}
	}

	// the request for 25 should fail. See:
	// server: WARN  peer requested unknown block by number   num=25
	// client: WARN  failed p2p sync request    num=25 err="peer failed to serve request with code 1"
	require.Zero(t, len(recvB), "there is a gap, should not see other payloads yet")

	// race-condition fix: the request for 25 is expected to error, but is marked as complete in the peer-loop.
	// But the re-request checks the status in the main loop, and it may thus look like it's still in-flight,
	// and thus not run the new request.
	// Wait till the failed request is recognized as marked as done, so the re-request actually runs.
	ctx, cancelFunc = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelFunc()
	for {
		isInFlight, err := clB.isInFlight(ctx, 25)
		require.NoError(t, err)
		time.Sleep(time.Second)
		if !isInFlight {
			break
		}
	}
	require.False(t, clB.activeRangeRequests.get(rangeReqId), "expecting range request to be cancelled")

	// Add back the block
	payloads.addPayload(bl25)
	// And request a range again, 25 is there now, and 21-24 should follow quickly (some may already have been fetched and wait in quarantine)
	_, err = clB.RequestL2Range(ctx, payloads.getBlockRef(20), payloads.getBlockRef(26))
	require.NoError(t, err)
	for i := uint64(25); i > 20; i-- {
		p := <-recvB
		exp, ok := payloads.getPayload(uint64(p.ExecutionPayload.BlockNumber))
		require.True(t, ok, "expecting known payload")
		require.Equal(t, exp.ExecutionPayload.BlockHash, p.ExecutionPayload.BlockHash, "expecting the correct payload")
		require.Equal(t, exp.ParentBeaconBlockRoot, p.ParentBeaconBlockRoot)
		if cfg.IsEcotone(uint64(p.ExecutionPayload.Timestamp)) {
			require.NotNil(t, p.ParentBeaconBlockRoot)
		} else {
			require.Nil(t, p.ParentBeaconBlockRoot)
		}
	}
}

func TestNetworkNotifyAddPeerAndRemovePeer(t *testing.T) {
	t.Parallel()
	log := testlog.Logger(t, log.LevelDebug)

	cfg, _ := setupSyncTestData(25)

	confA := TestingConfig(t)
	confB := TestingConfig(t)
	hostA, err := confA.Host(log.New("host", "A"), nil, metrics.NoopMetrics)
	require.NoError(t, err, "failed to launch host A")
	defer hostA.Close()
	hostB, err := confB.Host(log.New("host", "B"), nil, metrics.NoopMetrics)
	require.NoError(t, err, "failed to launch host B")
	defer hostB.Close()

	syncCl := NewSyncClient(log, cfg, hostA, func(ctx context.Context, from peer.ID, payload *eth.ExecutionPayloadEnvelope) error {
		return nil
	}, metrics.NoopMetrics, &NoopApplicationScorer{})

	waitChan := make(chan struct{}, 2)
	var connectedOnce sync.Once
	var disconnectedOnce sync.Once
	hostA.Network().Notify(&network.NotifyBundle{
		ConnectedF: func(nw network.Network, conn network.Conn) {
			connectedOnce.Do(func() {
				syncCl.AddPeer(conn.RemotePeer())
				waitChan <- struct{}{}
			})
		},
		DisconnectedF: func(nw network.Network, conn network.Conn) {
			disconnectedOnce.Do(func() {
				// only when no connection is available, we can remove the peer
				if nw.Connectedness(conn.RemotePeer()) == network.NotConnected {
					syncCl.RemovePeer(conn.RemotePeer())
				}
				waitChan <- struct{}{}
			})
		},
	})
	syncCl.Start()

	err = hostA.Connect(context.Background(), peer.AddrInfo{ID: hostB.ID(), Addrs: hostB.Addrs()})
	require.NoError(t, err, "failed to connect to peer B from peer A")
	require.Equal(t, hostA.Network().Connectedness(hostB.ID()), network.Connected)

	// wait for async add process done
	<-waitChan
	_, ok := syncCl.peers[hostB.ID()]
	require.True(t, ok, "peerB should exist in syncClient")

	err = hostA.Network().ClosePeer(hostB.ID())
	require.NoError(t, err, "close peer fail")

	// wait for async removing process done
	<-waitChan
	syncCl.peersLock.Lock()
	// Technically this can't fail since SyncClient.RemovePeer also deletes from the
	// SyncClient.peers, so unless that action is deferred to SyncClient.peerLoop it's not very
	// interesting.
	_, peerBExist3 := syncCl.peers[hostB.ID()]
	syncCl.peersLock.Unlock()
	require.False(t, peerBExist3, "peerB should not exist in syncClient")
}

func TestPanicGuard(t *testing.T) {
	mockPanickingFn := func(ctx context.Context, id peer.ID, expectedBlockNum uint64) error {
		panic("gotcha")
	}
	require.NotPanics(t, func() {
		err := panicGuard(mockPanickingFn)(context.Background(), peer.ID(""), 37)
		require.EqualError(t, err, "recovered from a panic: gotcha")
	})
}

func TestRequestResultErr_Error(t *testing.T) {
	for _, test := range []struct {
		code   byte
		expStr string
	}{
		{
			code:   0,
			expStr: "success",
		},
		{
			code:   1,
			expStr: "not found",
		},
		{
			code:   2,
			expStr: "invalid request",
		},
		{
			code:   3,
			expStr: "unknown error",
		},
		{
			code:   4,
			expStr: "invalid code",
		},
		{
			code:   0xff,
			expStr: "invalid code",
		},
	} {
		t.Run(fmt.Sprintf("code %d", test.code), func(t *testing.T) {
			err := requestResultErr(test.code)
			errStr := err.Error()
			if !strings.HasSuffix(errStr, test.expStr) {
				t.Fatalf("unexpected error string %q, expted suffix %q", errStr, test.expStr)
			}
		})
	}
}
