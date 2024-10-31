package p2p

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/ethereum-optimism/optimism/op-node/p2p/gating"

	decredSecp "github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/ethereum-optimism/optimism/op-node/p2p/store"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p-testing/netutil"
	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"

	gcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/enode"

	"github.com/ethereum-optimism/optimism/op-node/metrics"
)

// TODO: dynamic peering
// - req-resp protocol to ensure peers from a different chain learn they shouldn't be connected
// - banning peers based on score

var (
	ErrDisabledDiscovery   = errors.New("discovery disabled")
	ErrNoConnectionManager = errors.New("no connection manager")
	ErrNoConnectionGater   = errors.New("no connection gater")
	ErrInvalidRequest      = errors.New("invalid request")
)

type Node interface {
	// Host returns the libp2p host
	Host() host.Host
	// Dv5Local returns the control over the Discv5 data of the local node, nil if disabled
	Dv5Local() *enode.LocalNode
	// Dv5Udp returns the control over the Discv5 network, nil if disabled
	Dv5Udp() *discover.UDPv5
	// GossipSub returns the gossip router
	GossipSub() *pubsub.PubSub
	// GossipOut returns the gossip output/info control
	GossipOut() GossipOut
	// ConnectionGater returns the connection gater, to ban/unban peers with, may be nil
	ConnectionGater() gating.BlockingConnectionGater
	// ConnectionManager returns the connection manager, to protect peers with, may be nil
	ConnectionManager() connmgr.ConnManager
}

type APIBackend struct {
	node Node
	log  log.Logger
	m    metrics.Metricer
}

var _ API = (*APIBackend)(nil)

func NewP2PAPIBackend(node Node, log log.Logger, m metrics.Metricer) *APIBackend {
	if m == nil {
		m = metrics.NoopMetrics
	}

	return &APIBackend{
		node: node,
		log:  log,
		m:    m,
	}
}

func (s *APIBackend) Self(ctx context.Context) (*PeerInfo, error) {
	recordDur := s.m.RecordRPCServerRequest("opp2p_self")
	defer recordDur()
	h := s.node.Host()
	nw := h.Network()
	pstore := h.Peerstore()
	info, err := dumpPeer(h.ID(), nw, pstore, s.node.ConnectionManager())
	if err != nil {
		return nil, err
	}
	info.GossipBlocks = true
	info.Latency = 0
	if local := s.node.Dv5Local(); local != nil {
		info.ENR = local.Node().String()
	}
	return info, nil
}

func dumpPeer(id peer.ID, nw network.Network, pstore peerstore.Peerstore, connMgr connmgr.ConnManager) (*PeerInfo, error) {
	info := &PeerInfo{
		PeerID: id,
	}

	// we might not have the pubkey if it's from a multi-addr and if we never discovered/connected them
	pub := pstore.PubKey(id)
	if pub != nil {
		if testPub, ok := pub.(netutil.TestBogusPublicKey); ok {
			info.NodeID = enode.ID(gcrypto.Keccak256Hash(testPub))
		} else {
			typedPub, ok := pub.(*crypto.Secp256k1PublicKey)
			if !ok {
				return nil, fmt.Errorf("unexpected pubkey type: %T", pub)
			}
			info.NodeID = enode.PubkeyToIDV4((*decredSecp.PublicKey)(typedPub).ToECDSA())
		}
	}
	if eps, ok := pstore.(store.ExtendedPeerstore); ok {
		if dat, err := eps.GetPeerScores(id); err == nil {
			info.PeerScores = dat
		}
		if md, err := eps.GetPeerMetadata(id); err == nil {
			info.ENR = md.ENR
			info.ChainID = md.OPStackID
		}
	}
	if dat, err := pstore.Get(id, "ProtocolVersion"); err == nil {
		protocolVersion, ok := dat.(string)
		if ok {
			info.ProtocolVersion = protocolVersion
		}
	}
	if dat, err := pstore.Get(id, "AgentVersion"); err == nil {
		agentVersion, ok := dat.(string)
		if ok {
			info.UserAgent = agentVersion
		}
	}
	// include the /p2p/ address component in all of the addresses for convenience of the API user.
	p2pAddrs, err := peer.AddrInfoToP2pAddrs(&peer.AddrInfo{ID: id, Addrs: pstore.Addrs(id)})
	if err == nil {
		for _, addr := range p2pAddrs {
			info.Addresses = append(info.Addresses, addr.String())
		}
	}
	info.Connectedness = nw.Connectedness(id)
	if protocols, err := pstore.GetProtocols(id); err == nil {
		for _, id := range protocols {
			info.Protocols = append(info.Protocols, string(id))
		}
	}
	// get the first connection direction, if any (will default to unknown when there are no connections)
	for _, c := range nw.ConnsToPeer(id) {
		info.Direction = c.Stat().Direction
		break
	}
	info.Latency = pstore.LatencyEWMA(id)
	if connMgr != nil {
		info.Protected = connMgr.IsProtected(id, "")
	}

	return info, nil
}

// Peers lists information of peers. Optionally filter to only retrieve connected peers.
func (s *APIBackend) Peers(ctx context.Context, connected bool) (*PeerDump, error) {
	recordDur := s.m.RecordRPCServerRequest("opp2p_peers")
	defer recordDur()
	h := s.node.Host()
	nw := h.Network()
	pstore := h.Peerstore()
	var peers []peer.ID
	if connected {
		peers = nw.Peers()
	} else {
		peers = pstore.Peers()
	}

	dump := &PeerDump{Peers: make(map[string]*PeerInfo)}
	for _, id := range peers {
		peerInfo, err := dumpPeer(id, nw, pstore, s.node.ConnectionManager())
		if err != nil {
			s.log.Debug("failed to dump peer info in RPC request", "peer", id, "err", err)
			continue
		}
		// We don't use the peer.ID type as key,
		// since JSON decoding can't use the provided json unmarshaler (on *string type).
		dump.Peers[id.String()] = peerInfo
		if peerInfo.Connectedness == network.Connected {
			dump.TotalConnected += 1
		}
	}
	for _, id := range s.node.GossipOut().AllBlockTopicsPeers() {
		if p, ok := dump.Peers[id.String()]; ok {
			p.GossipBlocks = true
		}
	}
	if gater := s.node.ConnectionGater(); gater != nil {
		dump.BannedPeers = gater.ListBlockedPeers()
		dump.BannedSubnets = gater.ListBlockedSubnets()
		dump.BannedIPS = gater.ListBlockedAddrs()
	}
	return dump, nil
}

type PeerStats struct {
	Connected     uint `json:"connected"`
	Table         uint `json:"table"`
	BlocksTopic   uint `json:"blocksTopic"`
	BlocksTopicV2 uint `json:"blocksTopicV2"`
	BlocksTopicV3 uint `json:"blocksTopicV3"`
	BlocksTopicV4 uint `json:"blocksTopicV4"`
	Banned        uint `json:"banned"`
	Known         uint `json:"known"`
}

func (s *APIBackend) PeerStats(_ context.Context) (*PeerStats, error) {
	recordDur := s.m.RecordRPCServerRequest("opp2p_peerStats")
	defer recordDur()
	h := s.node.Host()
	nw := h.Network()
	pstore := h.Peerstore()

	stats := &PeerStats{
		Connected:     uint(len(nw.Peers())),
		Table:         0,
		BlocksTopic:   uint(len(s.node.GossipOut().BlocksTopicV1Peers())),
		BlocksTopicV2: uint(len(s.node.GossipOut().BlocksTopicV2Peers())),
		BlocksTopicV3: uint(len(s.node.GossipOut().BlocksTopicV3Peers())),
		BlocksTopicV4: uint(len(s.node.GossipOut().BlocksTopicV4Peers())),
		Banned:        0,
		Known:         uint(len(pstore.Peers())),
	}
	if gater := s.node.ConnectionGater(); gater != nil {
		stats.Banned = uint(len(gater.ListBlockedPeers()))
	}
	if dv5 := s.node.Dv5Udp(); dv5 != nil {
		stats.Table = uint(len(dv5.AllNodes()))
	}
	return stats, nil
}

func (s *APIBackend) DiscoveryTable(_ context.Context) ([]*enode.Node, error) {
	recordDur := s.m.RecordRPCServerRequest("opp2p_discoveryTable")
	defer recordDur()
	if dv5 := s.node.Dv5Udp(); dv5 != nil {
		return dv5.AllNodes(), nil
	} else {
		return nil, ErrDisabledDiscovery
	}
}

func (s *APIBackend) BlockPeer(_ context.Context, id peer.ID) error {
	recordDur := s.m.RecordRPCServerRequest("opp2p_blockPeer")
	if err := id.Validate(); err != nil {
		s.log.Warn("invalid peer ID", "method", "BlockPeer", "peer", id, "err", err)
		return ErrInvalidRequest
	}
	defer recordDur()
	if gater := s.node.ConnectionGater(); gater == nil {
		return ErrNoConnectionGater
	} else {
		return gater.BlockPeer(id)
	}
}

func (s *APIBackend) UnblockPeer(_ context.Context, id peer.ID) error {
	recordDur := s.m.RecordRPCServerRequest("opp2p_unblockPeer")
	if err := id.Validate(); err != nil {
		s.log.Warn("invalid peer ID", "method", "UnblockPeer", "peer", id, "err", err)
		return ErrInvalidRequest
	}
	defer recordDur()
	if gater := s.node.ConnectionGater(); gater == nil {
		return ErrNoConnectionGater
	} else {
		return gater.UnblockPeer(id)
	}
}

func (s *APIBackend) ListBlockedPeers(_ context.Context) ([]peer.ID, error) {
	recordDur := s.m.RecordRPCServerRequest("opp2p_listBlockedPeers")
	defer recordDur()
	if gater := s.node.ConnectionGater(); gater == nil {
		return nil, ErrNoConnectionGater
	} else {
		return gater.ListBlockedPeers(), nil
	}
}

// BlockAddr adds an IP address to the set of blocked addresses.
// Note: active connections to the IP address are not automatically closed.
func (s *APIBackend) BlockAddr(_ context.Context, ip net.IP) error {
	recordDur := s.m.RecordRPCServerRequest("opp2p_blockAddr")
	if ip == nil {
		s.log.Warn("invalid IP", "method", "BlockAddr")
		return ErrInvalidRequest
	}
	defer recordDur()
	if gater := s.node.ConnectionGater(); gater == nil {
		return ErrNoConnectionGater
	} else {
		return gater.BlockAddr(ip)
	}
}

func (s *APIBackend) UnblockAddr(_ context.Context, ip net.IP) error {
	recordDur := s.m.RecordRPCServerRequest("opp2p_unblockAddr")
	if ip == nil {
		s.log.Warn("invalid IP", "method", "UnblockAddr")
		return ErrInvalidRequest
	}
	defer recordDur()
	if gater := s.node.ConnectionGater(); gater == nil {
		return ErrNoConnectionGater
	} else {
		return gater.UnblockAddr(ip)
	}
}

func (s *APIBackend) ListBlockedAddrs(_ context.Context) ([]net.IP, error) {
	recordDur := s.m.RecordRPCServerRequest("opp2p_listBlockedAddrs")
	defer recordDur()
	if gater := s.node.ConnectionGater(); gater == nil {
		return nil, ErrNoConnectionGater
	} else {
		return gater.ListBlockedAddrs(), nil
	}
}

// BlockSubnet adds an IP subnet to the set of blocked addresses.
// Note: active connections to the IP subnet are not automatically closed.
func (s *APIBackend) BlockSubnet(_ context.Context, ipnet *net.IPNet) error {
	recordDur := s.m.RecordRPCServerRequest("opp2p_blockSubnet")
	if ipnet == nil || ipnet.IP == nil || ipnet.Mask == nil {
		s.log.Warn("invalid IPNet", "method", "BlockSubnet")
		return ErrInvalidRequest
	}
	defer recordDur()
	if gater := s.node.ConnectionGater(); gater == nil {
		return ErrNoConnectionGater
	} else {
		return gater.BlockSubnet(ipnet)
	}
}

func (s *APIBackend) UnblockSubnet(_ context.Context, ipnet *net.IPNet) error {
	recordDur := s.m.RecordRPCServerRequest("opp2p_unblockSubnet")
	if ipnet == nil || ipnet.IP == nil || ipnet.Mask == nil {
		s.log.Warn("invalid IPNet", "method", "UnblockSubnet")
		return ErrInvalidRequest
	}
	defer recordDur()
	if gater := s.node.ConnectionGater(); gater == nil {
		return ErrNoConnectionGater
	} else {
		return gater.UnblockSubnet(ipnet)
	}
}

func (s *APIBackend) ListBlockedSubnets(_ context.Context) ([]*net.IPNet, error) {
	recordDur := s.m.RecordRPCServerRequest("opp2p_listBlockedSubnets")
	defer recordDur()
	if gater := s.node.ConnectionGater(); gater == nil {
		return nil, ErrNoConnectionGater
	} else {
		return gater.ListBlockedSubnets(), nil
	}
}

func (s *APIBackend) ProtectPeer(_ context.Context, id peer.ID) error {
	recordDur := s.m.RecordRPCServerRequest("opp2p_protectPeer")
	if err := id.Validate(); err != nil {
		s.log.Warn("invalid peer ID", "method", "ProtectPeer", "peer", id, "err", err)
		return ErrInvalidRequest
	}
	defer recordDur()
	if manager := s.node.ConnectionManager(); manager == nil {
		return ErrNoConnectionManager
	} else {
		manager.Protect(id, "api-protected")
		return nil
	}
}

func (s *APIBackend) UnprotectPeer(_ context.Context, id peer.ID) error {
	recordDur := s.m.RecordRPCServerRequest("opp2p_unprotectPeer")
	if err := id.Validate(); err != nil {
		s.log.Warn("invalid peer ID", "method", "UnprotectPeer", "peer", id, "err", err)
		return ErrInvalidRequest
	}
	defer recordDur()
	if manager := s.node.ConnectionManager(); manager == nil {
		return ErrNoConnectionManager
	} else {
		manager.Unprotect(id, "api-protected")
		return nil
	}
}

// ConnectPeer connects to a given peer address, and wait for protocol negotiation & identification of the peer
func (s *APIBackend) ConnectPeer(ctx context.Context, addr string) error {
	recordDur := s.m.RecordRPCServerRequest("opp2p_connectPeer")
	defer recordDur()
	h := s.node.Host()
	addrInfo, err := peer.AddrInfoFromString(addr)
	if err != nil {
		return fmt.Errorf("bad peer address: %w", err)
	}
	// Put a sanity limit on the connection time
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()
	return h.Connect(ctx, *addrInfo)
}

func (s *APIBackend) DisconnectPeer(_ context.Context, id peer.ID) error {
	recordDur := s.m.RecordRPCServerRequest("opp2p_disconnectPeer")
	if err := id.Validate(); err != nil {
		s.log.Warn("invalid peer ID", "method", "DisconnectPeer", "peer", id, "err", err)
		return ErrInvalidRequest
	}
	defer recordDur()
	err := s.node.Host().Network().ClosePeer(id)
	if err != nil {
		return err
	}
	ps := s.node.Host().Peerstore()
	ps.RemovePeer(id)
	ps.ClearAddrs(id)
	err = s.node.ConnectionGater().UnblockPeer(id)
	if err != nil {
		return fmt.Errorf("closed peer but failed to unblock: %w", err)
	}
	return nil
}
