package p2p

import (
	"context"
	"crypto/rand"
	"math/big"
	"net"
	"slices"
	gosync "sync"
	"testing"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-node/metrics"
	"github.com/ethereum-optimism/optimism/op-node/p2p/store"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
)

func TestingConfig(t *testing.T) *Config {
	p, _, err := crypto.GenerateSecp256k1Key(rand.Reader)
	require.NoError(t, err, "failed to generate new p2p priv key")

	return &Config{
		Priv:                (p).(*crypto.Secp256k1PrivateKey),
		DisableP2P:          false,
		NoDiscovery:         true, // we statically peer during most tests.
		ListenIP:            net.IP{127, 0, 0, 1},
		ListenTCPPort:       0, // bind to any available port
		StaticPeers:         nil,
		HostMux:             []libp2p.Option{YamuxC()},
		NoTransportSecurity: true,
		PeersLo:             1,
		PeersHi:             10,
		PeersGrace:          time.Second * 10,
		NAT:                 false,
		UserAgent:           "optimism-testing",
		TimeoutNegotiation:  time.Second * 2,
		TimeoutAccept:       time.Second * 2,
		TimeoutDial:         time.Second * 2,
		Store:               sync.MutexWrap(ds.NewMapDatastore()),
	}
}

// Simplified p2p test, to debug/test basic libp2p things with
func TestP2PSimple(t *testing.T) {
	confA := TestingConfig(t)
	confB := TestingConfig(t)
	hostA, err := confA.Host(testlog.Logger(t, log.LevelError).New("host", "A"), nil, metrics.NoopMetrics)
	require.NoError(t, err, "failed to launch host A")
	defer hostA.Close()
	hostB, err := confB.Host(testlog.Logger(t, log.LevelError).New("host", "B"), nil, metrics.NoopMetrics)
	require.NoError(t, err, "failed to launch host B")
	defer hostB.Close()
	err = hostA.Connect(context.Background(), peer.AddrInfo{ID: hostB.ID(), Addrs: hostB.Addrs()})
	require.NoError(t, err, "failed to connect to peer B from peer A")
	require.Equal(t, hostB.Network().Connectedness(hostA.ID()), network.Connected)
}

type mockGossipIn struct {
	OnUnsafeL2PayloadFn func(ctx context.Context, from peer.ID, msg *eth.ExecutionPayloadEnvelope) error
}

func (m *mockGossipIn) OnUnsafeL2Payload(ctx context.Context, from peer.ID, msg *eth.ExecutionPayloadEnvelope) error {
	if m.OnUnsafeL2PayloadFn != nil {
		return m.OnUnsafeL2PayloadFn(ctx, from, msg)
	}
	return nil
}

// Full setup, using negotiated transport security and muxes
func TestP2PFull(t *testing.T) {
	pA, _, err := crypto.GenerateSecp256k1Key(rand.Reader)
	require.NoError(t, err, "failed to generate new p2p priv key")
	pB, _, err := crypto.GenerateSecp256k1Key(rand.Reader)
	require.NoError(t, err, "failed to generate new p2p priv key")

	confA := Config{
		Priv:                (pA).(*crypto.Secp256k1PrivateKey),
		DisableP2P:          false,
		NoDiscovery:         true,
		ListenIP:            net.IP{127, 0, 0, 1},
		ListenTCPPort:       0, // bind to any available port
		StaticPeers:         nil,
		HostMux:             []libp2p.Option{YamuxC(), MplexC()},
		HostSecurity:        []libp2p.Option{NoiseC(), TlsC()},
		NoTransportSecurity: false,
		PeersLo:             1,
		PeersHi:             10,
		PeersGrace:          time.Second * 10,
		NAT:                 false,
		UserAgent:           "optimism-testing",
		TimeoutNegotiation:  time.Second * 2,
		TimeoutAccept:       time.Second * 2,
		TimeoutDial:         time.Second * 2,
		Store:               sync.MutexWrap(ds.NewMapDatastore()),
	}
	// copy config A, and change the settings for B
	confB := confA
	confB.Priv = (pB).(*crypto.Secp256k1PrivateKey)
	confB.Store = sync.MutexWrap(ds.NewMapDatastore())
	// TODO: maybe swap the order of sec/mux preferences, to test that negotiation works

	runCfgA := &testutils.MockRuntimeConfig{P2PSeqAddress: common.Address{0x42}}
	runCfgB := &testutils.MockRuntimeConfig{P2PSeqAddress: common.Address{0x42}}

	logA := testlog.Logger(t, log.LevelError).New("host", "A")
	nodeA, err := NewNodeP2P(context.Background(), &rollup.Config{}, logA, &confA, &mockGossipIn{}, nil, runCfgA, metrics.NoopMetrics, false)
	require.NoError(t, err)
	defer nodeA.Close()

	conns := make(chan network.Conn, 1)
	hostA := nodeA.Host()
	var once gosync.Once
	hostA.Network().Notify(&network.NotifyBundle{
		ConnectedF: func(n network.Network, conn network.Conn) {
			once.Do(func() {
				conns <- conn
			})
		}})

	backend := NewP2PAPIBackend(nodeA, logA, nil)
	srv := rpc.NewServer()
	require.NoError(t, srv.RegisterName("opp2p", backend))
	client := rpc.DialInProc(srv)
	p2pClientA := NewClient(client)

	// Set up B to connect statically
	confB.StaticPeers, err = peer.AddrInfoToP2pAddrs(&peer.AddrInfo{ID: hostA.ID(), Addrs: hostA.Addrs()})
	require.NoError(t, err)

	// Add address of host B itself, it shouldn't connect or cause issues.
	idB, err := peer.IDFromPublicKey(confB.Priv.GetPublic())
	require.NoError(t, err)
	altAddrB, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/12345/p2p/" + idB.String())
	require.NoError(t, err)
	confB.StaticPeers = append(confB.StaticPeers, altAddrB)

	logB := testlog.Logger(t, log.LevelError).New("host", "B")

	nodeB, err := NewNodeP2P(context.Background(), &rollup.Config{}, logB, &confB, &mockGossipIn{}, nil, runCfgB, metrics.NoopMetrics, false)
	require.NoError(t, err)
	defer nodeB.Close()
	hostB := nodeB.Host()

	require.True(t, nodeB.IsStatic(hostA.ID()), "node A must be static peer of node B")
	require.False(t, nodeB.IsStatic(hostB.ID()), "node B must not be static peer of node B itself")

	select {
	case <-time.After(30 * time.Second):
		t.Fatal("failed to connect new host")
	case c := <-conns:
		require.Equal(t, hostB.ID(), c.RemotePeer())
	}

	ctx := context.Background()

	selfInfoA, err := p2pClientA.Self(ctx)
	require.NoError(t, err)
	require.Equal(t, selfInfoA.PeerID, hostA.ID())

	_, err = p2pClientA.DiscoveryTable(ctx)
	// rpc does not preserve error type
	require.Equal(t, err.Error(), ErrDisabledDiscovery.Error(), "expecting discv5 to be disabled")

	require.NoError(t, p2pClientA.BlockPeer(ctx, hostB.ID()))
	blockedPeers, err := p2pClientA.ListBlockedPeers(ctx)
	require.NoError(t, err)
	require.Equal(t, []peer.ID{hostB.ID()}, blockedPeers)
	require.NoError(t, p2pClientA.UnblockPeer(ctx, hostB.ID()))

	require.Error(t, p2pClientA.BlockAddr(ctx, nil))
	require.Error(t, p2pClientA.UnblockAddr(ctx, nil))

	require.Error(t, p2pClientA.BlockSubnet(ctx, nil))
	require.Error(t, p2pClientA.BlockSubnet(ctx, &net.IPNet{}))
	require.Error(t, p2pClientA.BlockSubnet(ctx, &net.IPNet{Mask: net.IPMask{255, 255, 0, 0}}))
	require.Error(t, p2pClientA.BlockSubnet(ctx, &net.IPNet{IP: net.IP{0, 0, 0, 1}}))
	require.NoError(t, p2pClientA.BlockSubnet(ctx, &net.IPNet{IP: net.IP{0, 0, 0, 1}, Mask: net.IPMask{255, 255, 0, 0}}))

	require.Error(t, p2pClientA.UnblockSubnet(ctx, nil))
	require.Error(t, p2pClientA.UnblockSubnet(ctx, &net.IPNet{}))
	require.Error(t, p2pClientA.UnblockSubnet(ctx, &net.IPNet{Mask: net.IPMask{255, 255, 0, 0}}))
	require.Error(t, p2pClientA.UnblockSubnet(ctx, &net.IPNet{IP: net.IP{0, 0, 0, 1}}))
	require.NoError(t, p2pClientA.UnblockSubnet(ctx, &net.IPNet{IP: net.IP{0, 0, 0, 1}, Mask: net.IPMask{255, 255, 0, 0}}))

	require.Error(t, p2pClientA.BlockPeer(ctx, ""))
	require.Error(t, p2pClientA.UnblockPeer(ctx, ""))
	require.Error(t, p2pClientA.ProtectPeer(ctx, ""))
	require.Error(t, p2pClientA.UnprotectPeer(ctx, ""))
	require.Error(t, p2pClientA.ConnectPeer(ctx, ""))
	require.Error(t, p2pClientA.DisconnectPeer(ctx, ""))

	require.NoError(t, p2pClientA.BlockAddr(ctx, net.IP{123, 123, 123, 123}))
	blockedIPs, err := p2pClientA.ListBlockedAddrs(ctx)
	require.NoError(t, err)
	require.Len(t, blockedIPs, 1)
	require.Equal(t, net.IP{123, 123, 123, 123}, blockedIPs[0].To4())
	require.NoError(t, p2pClientA.UnblockAddr(ctx, net.IP{123, 123, 123, 123}))

	subnet := &net.IPNet{IP: net.IP{123, 0, 0, 0}.To16(), Mask: net.IPMask{0xff, 0, 0, 0}}
	require.NoError(t, p2pClientA.BlockSubnet(ctx, subnet))
	blockedSubnets, err := p2pClientA.ListBlockedSubnets(ctx)
	require.NoError(t, err)
	require.Len(t, blockedSubnets, 1)
	require.Equal(t, subnet, blockedSubnets[0])
	require.NoError(t, p2pClientA.UnblockSubnet(ctx, subnet))

	// Ask host A for all peer information they have
	peerDump, err := p2pClientA.Peers(ctx, false)
	require.Nil(t, err)
	require.Contains(t, peerDump.Peers, hostB.ID().String())
	data := peerDump.Peers[hostB.ID().String()]
	require.Equal(t, data.Direction, network.DirInbound)

	stats, err := p2pClientA.PeerStats(ctx)
	require.Nil(t, err)
	require.Equal(t, uint(1), stats.Connected)

	// disconnect
	hostBId := hostB.ID().String()
	peerDump, err = p2pClientA.Peers(ctx, false)
	require.Nil(t, err)
	data = peerDump.Peers[hostBId]
	require.NotNil(t, data)
	retries := 0
	for {
		require.NoError(t, p2pClientA.DisconnectPeer(ctx, hostB.ID()))
		// disconnect may take some time which we cant control from here
		// so we retry a few times increasing our wait tolerance
		time.Sleep(time.Duration(retries) * time.Second)
		peerDump, err = p2pClientA.Peers(ctx, false)
		require.Nil(t, err)
		data = peerDump.Peers[hostBId]
		if data == nil {
			break
		}
		retries++
		if retries > 3 {
			t.Fatal("failed to disconnect peer")
		}
	}
	require.Nil(t, data)

	// reconnect
	addrsB, err := peer.AddrInfoToP2pAddrs(&peer.AddrInfo{ID: hostB.ID(), Addrs: hostB.Addrs()})
	require.NoError(t, err)
	require.NoError(t, p2pClientA.ConnectPeer(ctx, addrsB[0].String()))

	require.NoError(t, p2pClientA.ProtectPeer(ctx, hostB.ID()))
	require.NoError(t, p2pClientA.UnprotectPeer(ctx, hostB.ID()))
}

func TestDiscovery(t *testing.T) {
	t.Skipf("skipping flaky test")

	pA, _, err := crypto.GenerateSecp256k1Key(rand.Reader)
	require.NoError(t, err, "failed to generate new p2p priv key")
	pB, _, err := crypto.GenerateSecp256k1Key(rand.Reader)
	require.NoError(t, err, "failed to generate new p2p priv key")
	pC, _, err := crypto.GenerateSecp256k1Key(rand.Reader)
	require.NoError(t, err, "failed to generate new p2p priv key")

	logA := testlog.Logger(t, log.LevelError).New("host", "A")
	logB := testlog.Logger(t, log.LevelError).New("host", "B")
	logC := testlog.Logger(t, log.LevelError).New("host", "C")

	discDBA, err := enode.OpenDB("") // "" = memory db
	require.NoError(t, err)
	discDBB, err := enode.OpenDB("")
	require.NoError(t, err)
	discDBC, err := enode.OpenDB("")
	require.NoError(t, err)

	rollupCfg := &rollup.Config{L2ChainID: big.NewInt(901)}

	confA := Config{
		Priv:                (pA).(*crypto.Secp256k1PrivateKey),
		DisableP2P:          false,
		NoDiscovery:         false,
		AdvertiseIP:         net.IP{127, 0, 0, 1},
		ListenUDPPort:       0, // bind to any available port
		ListenIP:            net.IP{127, 0, 0, 1},
		ListenTCPPort:       0, // bind to any available port
		StaticPeers:         nil,
		HostMux:             []libp2p.Option{YamuxC(), MplexC()},
		HostSecurity:        []libp2p.Option{NoiseC(), TlsC()},
		NoTransportSecurity: false,
		PeersLo:             1,
		PeersHi:             10,
		PeersGrace:          time.Second * 10,
		NAT:                 false,
		UserAgent:           "optimism-testing",
		TimeoutNegotiation:  time.Second * 2,
		TimeoutAccept:       time.Second * 2,
		TimeoutDial:         time.Second * 2,
		Store:               sync.MutexWrap(ds.NewMapDatastore()),
		DiscoveryDB:         discDBA,
	}
	// copy config A, and change the settings for B
	confB := confA
	confB.Priv = (pB).(*crypto.Secp256k1PrivateKey)
	confB.Store = sync.MutexWrap(ds.NewMapDatastore())
	confB.DiscoveryDB = discDBB

	runCfgA := &testutils.MockRuntimeConfig{P2PSeqAddress: common.Address{0x42}}
	runCfgB := &testutils.MockRuntimeConfig{P2PSeqAddress: common.Address{0x42}}
	runCfgC := &testutils.MockRuntimeConfig{P2PSeqAddress: common.Address{0x42}}

	resourcesCtx, resourcesCancel := context.WithCancel(context.Background())
	defer resourcesCancel()

	nodeA, err := NewNodeP2P(context.Background(), rollupCfg, logA, &confA, &mockGossipIn{}, nil, runCfgA, metrics.NoopMetrics, false)
	require.NoError(t, err)
	defer nodeA.Close()
	hostA := nodeA.Host()
	go nodeA.DiscoveryProcess(resourcesCtx, logA, rollupCfg, 10)

	// Add A as bootnode to B
	confB.Bootnodes = []*enode.Node{nodeA.Dv5Udp().Self()}
	// Copy B config to C, and ensure they have a different priv / peerstore
	confC := confB
	confC.Priv = (pC).(*crypto.Secp256k1PrivateKey)
	confC.Store = sync.MutexWrap(ds.NewMapDatastore())
	confB.DiscoveryDB = discDBC

	// Start B
	nodeB, err := NewNodeP2P(context.Background(), rollupCfg, logB, &confB, &mockGossipIn{}, nil, runCfgB, metrics.NoopMetrics, false)
	require.NoError(t, err)
	defer nodeB.Close()
	hostB := nodeB.Host()
	go nodeB.DiscoveryProcess(resourcesCtx, logB, rollupCfg, 10)

	// Track connections to B
	connsB := make(chan network.Conn, 2)
	hostB.Network().Notify(&network.NotifyBundle{
		ConnectedF: func(n network.Network, conn network.Conn) {
			log.Info("connection to B", "peer", conn.RemotePeer())
			connsB <- conn
		}})

	// Start C
	nodeC, err := NewNodeP2P(context.Background(), rollupCfg, logC, &confC, &mockGossipIn{}, nil, runCfgC, metrics.NoopMetrics, false)
	require.NoError(t, err)
	defer nodeC.Close()
	hostC := nodeC.Host()
	go nodeC.DiscoveryProcess(resourcesCtx, logC, rollupCfg, 10)

	// B and C don't know each other yet, but both have A as a bootnode.
	// It should only be a matter of time for them to connect, if they discover each other via A.
	timeout := time.After(time.Second * 60)
	var peersOfB []peer.ID
	// B should be connected to the bootnode (A) it used (it's a valid optimism node to connect to here)
	// C should also be connected, although this one might take more time to discover
	for !slices.Contains(peersOfB, hostA.ID()) || !slices.Contains(peersOfB, hostC.ID()) {
		select {
		case <-timeout:
			var peers []string
			for _, id := range peersOfB {
				peers = append(peers, id.String())
			}
			t.Fatalf("timeout reached - expected host A: %v and host C: %v to be in %v", hostA.ID().String(), hostC.ID().String(), peers)
		case c := <-connsB:
			peersOfB = append(peersOfB, c.RemotePeer())
		}
	}

	// Check that among known connections (B-A, B-C), we have metadata
	type mdcheck struct {
		n1 *NodeP2P
		n2 *NodeP2P
	}
	cases := []mdcheck{
		{nodeB, nodeA},
		{nodeB, nodeC},
	}
	for _, c := range cases {
		// make peerstore metadata available
		eps, ok := c.n1.Host().Peerstore().(store.ExtendedPeerstore)
		require.True(t, ok)
		// confirm n1 has metadata about n2
		md, err := eps.GetPeerMetadata(c.n2.Host().ID())
		require.NoError(t, err)
		require.NotEmpty(t, md.ENR)
		require.Equal(t, uint64(901), md.OPStackID)
	}
}

// Most tests should use mocknets instead of using the actual local host network
func TestP2PMocknet(t *testing.T) {
	mnet, err := mocknet.FullMeshConnected(3)
	require.NoError(t, err, "failed to setup mocknet")
	defer mnet.Close()
	hosts := mnet.Hosts()
	hostA, hostB, hostC := hosts[0], hosts[1], hosts[2]
	require.Equal(t, hostA.Network().Connectedness(hostB.ID()), network.Connected)
	require.Equal(t, hostA.Network().Connectedness(hostC.ID()), network.Connected)
	require.Equal(t, hostB.Network().Connectedness(hostC.ID()), network.Connected)
}
