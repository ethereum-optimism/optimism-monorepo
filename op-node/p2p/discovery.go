package p2p

import (
	"bytes"
	"context"
	secureRand "crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/netip"
	"time"

	decredSecp "github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"

	gcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/chaincfg/chainhash"

	"github.com/ethereum-optimism/optimism/op-node/p2p/store"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
)

// force to use the new chainhash module, and not the legacy chainhash package btcd module
const _ = chainhash.HashSize

// force to use the btcd module, while the keycard dependency in geth still depends on it,
// for go mod tidy to not clean up our explicit usage of v0.23.3, which resolves conflicts with the chainhash module
const _ = blockchain.CoinbaseWitnessDataLen

const (
	discoverIntervalFast   = time.Second * 5
	discoverIntervalSlow   = time.Second * 20
	connectionIntervalFast = time.Second * 5
	connectionIntervalSlow = time.Second * 20
	connectionWorkerCount  = 4
	connectionBufferSize   = 10
	discoveredNodesBuffer  = 3
	tableKickoffDelay      = time.Second * 3
	discoveredAddrTTL      = time.Hour * 24
	collectiveDialTimeout  = time.Second * 30
)

func resolveNodeDNS(n *enode.Node) (*enode.Node, error) {
	// Adapted from https://github.com/ethereum/go-ethereum/pull/30822/files#diff-cfca27b526255b13ca472b74f49acd227e8f9cfc866d8d05093c5658db0b4eb8R444
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	foundIPs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", n.Hostname())
	if err != nil {
		return n, err
	}

	// Check for IP updates.
	var (
		nodeIP4, nodeIP6   netip.Addr
		foundIP4, foundIP6 netip.Addr
	)
	n.Load((*enr.IPv4Addr)(&nodeIP4))
	n.Load((*enr.IPv6Addr)(&nodeIP6))
	for _, ip := range foundIPs {
		if ip.Is4() && !foundIP4.IsValid() {
			foundIP4 = ip
		}
		if ip.Is6() && !foundIP6.IsValid() {
			foundIP6 = ip
		}
	}

	if !foundIP4.IsValid() && !foundIP6.IsValid() {
		// Lookup failed.
		return n, fmt.Errorf("failed to resolve DNS hostname %q", n.Hostname())
	}
	if foundIP4 == nodeIP4 && foundIP6 == nodeIP6 {
		// No updates necessary.
		return n, nil
	}

	// Update the node. Note this invalidates the ENR signature, because we use SignNull
	// to create a modified copy. But this should be OK, since we just use the node as a
	// dial target. And nodes will usually only have a DNS hostname if they came from a
	// enode:// URL, which has no signature anyway. If it ever becomes a problem, the
	// resolved IP could also be stored into dialTask instead of the node.
	rec := n.Record()
	if foundIP4.IsValid() {
		rec.Set(enr.IPv4Addr(foundIP4))
	}
	if foundIP6.IsValid() {
		rec.Set(enr.IPv6Addr(foundIP6))
	}
	rec.SetSeq(n.Seq()) // ensure seq not bumped by update
	newNode := enode.SignNull(rec, n.ID()).WithHostname(n.Hostname())
	return newNode, nil
}

func resolveDNSOrFilter(bootnodes []*enode.Node) []*enode.Node {
	var resolved []*enode.Node
	for _, n := range bootnodes {
		if n.IP() != nil {
			resolved = append(resolved, n)
		} else if n.Hostname() != "" {
			newNode, err := resolveNodeDNS(n)
			if err != nil {
				log.Warn("failed to resolve bootnode DNS", "node", n.ID(), "err", err)
				continue
			}
			resolved = append(resolved, newNode)
		}
	}
	return resolved
}

func (conf *Config) Discovery(log log.Logger, rollupCfg *rollup.Config, tcpPort uint16) (*enode.LocalNode, *discover.UDPv5, error) {
	if conf.NoDiscovery {
		return nil, nil, nil
	}
	priv := (*decredSecp.PrivateKey)(conf.Priv).ToECDSA()
	// use the geth curve definition. Same crypto, but geth needs to detect it as *their* definition of the curve.
	priv.Curve = gcrypto.S256()
	localNode := enode.NewLocalNode(conf.DiscoveryDB, priv)
	if conf.AdvertiseIP != nil {
		localNode.SetStaticIP(conf.AdvertiseIP)
	}
	if conf.AdvertiseUDPPort != 0 { // explicitly advertised port gets priority
		localNode.SetFallbackUDP(int(conf.AdvertiseUDPPort))
	} else if conf.ListenUDPPort != 0 { // otherwise default to the port we configured it to listen on
		localNode.SetFallbackUDP(int(conf.ListenUDPPort))
	}
	if conf.AdvertiseTCPPort != 0 { // explicitly advertised port gets priority
		localNode.Set(enr.TCP(conf.AdvertiseTCPPort))
	} else if tcpPort != 0 { // otherwise try to pick up whatever port LibP2P binded to (listen port, or dynamically picked)
		localNode.Set(enr.TCP(tcpPort))
	} else if conf.ListenTCPPort != 0 { // otherwise default to the port we configured it to listen on
		localNode.Set(enr.TCP(conf.ListenTCPPort))
	} else {
		return nil, nil, fmt.Errorf("no TCP port to put in discovery record")
	}
	dat := OpStackENRData{
		chainID: rollupCfg.L2ChainID.Uint64(),
		version: 0,
	}
	localNode.Set(&dat)

	udpAddr := &net.UDPAddr{
		IP:   conf.ListenIP,
		Port: int(conf.ListenUDPPort),
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, nil, err
	}
	if udpAddr.Port == 0 { // if we picked a port dynamically, then find the port we got, and update our node record
		localUDPAddr := conn.LocalAddr().(*net.UDPAddr)
		localNode.SetFallbackUDP(localUDPAddr.Port)
	}

	bootnodes := resolveDNSOrFilter(conf.Bootnodes)

	cfg := discover.Config{
		PrivateKey:   priv,
		NetRestrict:  conf.NetRestrict,
		Bootnodes:    bootnodes,
		Unhandled:    nil, // Not used in dv5
		Log:          log,
		ValidSchemes: enode.ValidSchemes,
	}
	udpV5, err := discover.ListenV5(conn, localNode, cfg)
	if err != nil {
		return nil, nil, err
	}

	log.Info("started discovery service", "enr", localNode.Node(), "id", localNode.ID())

	// TODO: periodically we can pull the external IP and TCP port from libp2p NAT service,
	// and add it as a statement to keep the localNode accurate (if we trust the NAT device more than the discv5 statements)

	return localNode, udpV5, nil
}

// Secp256k1 is like the geth Secp256k1 enr entry type, but using the libp2p pubkey representation instead
type Secp256k1 crypto.Secp256k1PublicKey

func (v Secp256k1) ENRKey() string { return "secp256k1" }

// EncodeRLP implements rlp.Encoder.
func (v Secp256k1) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, (*decredSecp.PublicKey)(&v).SerializeCompressed())
}

// DecodeRLP implements rlp.Decoder.
func (v *Secp256k1) DecodeRLP(s *rlp.Stream) error {
	buf, err := s.Bytes()
	if err != nil {
		return err
	}
	pk, err := decredSecp.ParsePubKey(buf)
	if err != nil {
		return err
	}
	*v = (Secp256k1)(*pk)
	return nil
}

func enrToAddrInfo(r *enode.Node) (*peer.AddrInfo, *crypto.Secp256k1PublicKey, error) {
	ip := r.IP()
	ipScheme := "ip4"
	if ip4 := ip.To4(); ip4 == nil {
		ipScheme = "ip6"
	} else {
		ip = ip4
	}
	mAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/%s/%s/tcp/%d", ipScheme, ip.String(), r.TCP()))
	if err != nil {
		return nil, nil, fmt.Errorf("could not construct multi addr: %w", err)
	}
	var enrPub Secp256k1
	if err := r.Load(&enrPub); err != nil {
		return nil, nil, fmt.Errorf("failed to load pubkey as libp2p pubkey type from ENR")
	}
	pub := (*crypto.Secp256k1PublicKey)(&enrPub)
	peerID, err := peer.IDFromPublicKey(pub)
	if err != nil {
		return nil, pub, fmt.Errorf("could not compute peer ID from pubkey for multi-addr: %w", err)
	}
	return &peer.AddrInfo{
		ID:    peerID,
		Addrs: []multiaddr.Multiaddr{mAddr},
	}, pub, nil
}

// The discovery ENRs are just key-value lists, and we filter them by records tagged with the "opstack" key,
// and then check the chain ID and version.
type OpStackENRData struct {
	chainID uint64
	version uint64
}

func (o *OpStackENRData) ENRKey() string {
	return "opstack"
}

func (o *OpStackENRData) EncodeRLP(w io.Writer) error {
	out := make([]byte, 2*binary.MaxVarintLen64)
	offset := binary.PutUvarint(out, o.chainID)
	offset += binary.PutUvarint(out[offset:], o.version)
	out = out[:offset]
	// encode as byte-string
	return rlp.Encode(w, out)
}

func (o *OpStackENRData) DecodeRLP(s *rlp.Stream) error {
	b, err := s.Bytes()
	if err != nil {
		return fmt.Errorf("failed to decode outer ENR entry: %w", err)
	}
	// We don't check the byte length: the below readers are limited, and the ENR itself has size limits.
	// Future "opstack" entries may contain additional data, and will be tagged with a newer version etc.
	r := bytes.NewReader(b)
	chainID, err := binary.ReadUvarint(r)
	if err != nil {
		return fmt.Errorf("failed to read chain ID var int: %w", err)
	}
	version, err := binary.ReadUvarint(r)
	if err != nil {
		return fmt.Errorf("failed to read version var int: %w", err)
	}
	o.chainID = chainID
	o.version = version
	return nil
}

var _ enr.Entry = (*OpStackENRData)(nil)

func FilterEnodes(log log.Logger, cfg *rollup.Config) func(node *enode.Node) bool {
	return func(node *enode.Node) bool {
		var dat OpStackENRData
		err := node.Load(&dat)
		// if the entry does not exist, or if it is invalid, then ignore the node
		if err != nil {
			log.Trace("discovered node record has no opstack info", "node", node.ID(), "err", err)
			return false
		}
		// check chain ID matches
		if cfg.L2ChainID.Uint64() != dat.chainID {
			log.Trace("discovered node record has no matching chain ID", "node", node.ID(), "got", dat.chainID, "expected", cfg.L2ChainID.Uint64())
			return false
		}
		// check version matches
		if dat.version != 0 {
			log.Trace("discovered node record has no matching version", "node", node.ID(), "got", dat.version, "expected", 0)
			return false
		}
		return true
	}
}

// DiscoveryProcess runs a discovery process that randomly walks the DHT to fill the peerstore,
// and connects to nodes in the peerstore that we are not already connected to.
// Nodes from the peerstore will be shuffled, unsuccessful connection attempts will cause peers to be avoided,
// and only nodes with addresses (under TTL) will be connected to.
func (n *NodeP2P) DiscoveryProcess(ctx context.Context, log log.Logger, cfg *rollup.Config, connectGoal uint) {
	if n.dv5Udp == nil {
		log.Warn("peer discovery is disabled")
		return
	}
	filter := FilterEnodes(log, cfg)
	// We pull nodes from discv5 DHT in random order to find new peers.
	// Eventually we'll find a peer record that matches our filter.
	randomNodeIter := n.dv5Udp.RandomNodes()

	randomNodeIter = enode.Filter(randomNodeIter, filter)
	defer randomNodeIter.Close()

	// We pull from the DHT in a slow/fast interval, depending on the need to find more peers
	discoverTicker := time.NewTicker(discoverIntervalFast)
	defer discoverTicker.Stop()

	// We connect to the peers we know of to maintain a target,
	// but do so with polling to avoid scanning the connection count continuously
	connectTicker := time.NewTicker(connectionIntervalFast)
	defer connectTicker.Stop()

	// We can go faster/slower depending on the need
	slower := func() {
		discoverTicker.Reset(discoverIntervalSlow)
		connectTicker.Reset(connectionIntervalSlow)
	}
	faster := func() {
		discoverTicker.Reset(discoverIntervalFast)
		connectTicker.Reset(connectionIntervalFast)
	}

	// We try to connect to peers in parallel: some may be slow to respond
	connAttempts := make(chan peer.ID, connectionBufferSize)
	connectWorker := func(ctx context.Context) {
		for {
			id, ok := <-connAttempts
			if !ok {
				return
			}
			addrs := n.Host().Peerstore().Addrs(id)
			log.Info("attempting connection", "peer", id)
			ctx, cancel := context.WithTimeout(ctx, time.Second*10)
			err := n.Host().Connect(ctx, peer.AddrInfo{ID: id, Addrs: addrs})
			cancel()
			if err != nil {
				log.Debug("failed connection attempt", "peer", id, "err", err)
			}
		}
	}

	// stops all the workers when we are done
	defer close(connAttempts)
	// start workers to try connect to peers
	for i := 0; i < connectionWorkerCount; i++ {
		go connectWorker(ctx)
	}

	// buffer discovered nodes, so don't stall on the dht iteration as much
	randomNodesCh := make(chan *enode.Node, discoveredNodesBuffer)
	defer close(randomNodesCh)
	bufferNodes := func() {
		for {
			select {
			case <-discoverTicker.C:
				if !randomNodeIter.Next() {
					log.Info("discv5 DHT iteration stopped, closing peer discovery now...")
					return
				}
				found := randomNodeIter.Node()
				select {
				// block once we have found enough nodes
				case randomNodesCh <- found:
					continue
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}
	// Walk the DHT in parallel, the discv5 interface does not use channels for the iteration
	go bufferNodes()

	// Kick off by trying the nodes we have in our table (previous nodes from last run and/or bootnodes)
	go func() {
		<-time.After(tableKickoffDelay)
		// At the start we might have trouble walking the DHT,
		// but we do have a table with some nodes,
		// so take the table and feed it into the discovery process
		for _, rec := range n.dv5Udp.AllNodes() {
			if filter(rec) {
				select {
				case randomNodesCh <- rec:
					continue
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	pstore := n.Host().Peerstore()
	for {
		select {
		case <-ctx.Done():
			log.Info("stopped peer discovery")
			return // no ctx error, expected close
		case found := <-randomNodesCh:
			var dat OpStackENRData
			if err := found.Load(&dat); err != nil { // we already filtered on chain ID and version
				continue
			}
			info, pub, err := enrToAddrInfo(found)
			if err != nil {
				continue
			}

			// record metadata to the peerstore if it is an extended peerstore
			if eps, ok := pstore.(store.ExtendedPeerstore); ok {
				_, err := eps.SetPeerMetadata(info.ID, store.PeerMetadata{
					ENR:       found.String(),
					OPStackID: dat.chainID,
				})
				if err != nil {
					log.Warn("failed to set peer metadata", "peer", info.ID, "err", err)
				}
			}
			// We add the addresses to the peerstore, and update the address TTL.
			// After that we stop using the address, assuming it may not be valid anymore (until we rediscover the node)
			pstore.AddAddrs(info.ID, info.Addrs, discoveredAddrTTL)
			_ = pstore.AddPubKey(info.ID, pub)

			// Tag the peer, we'd rather have the connection manager prune away old peers,
			// or peers on different chains, or anyone we have not seen via discovery.
			// There is no tag score decay yet, so just set it to 42.
			n.ConnectionManager().TagPeer(info.ID, fmt.Sprintf("opstack-%d-%d", dat.chainID, dat.version), 42)
			log.Debug("discovered peer", "peer", info.ID, "nodeID", found.ID(), "addr", info.Addrs[0])
		case <-connectTicker.C:
			connected := n.Host().Network().Peers()
			log.Debug("peering tick", "connected", len(connected),
				"advertised_udp", n.dv5Local.Node().UDP(),
				"advertised_tcp", n.dv5Local.Node().TCP(),
				"advertised_ip", n.dv5Local.Node().IP())
			if uint(len(connected)) < connectGoal {
				// Start looking for more peers more actively again
				faster()

				peersWithAddrs := n.Host().Peerstore().PeersWithAddrs()
				if err := shufflePeers(peersWithAddrs); err != nil {
					continue
				}

				existing := make(map[peer.ID]struct{})
				for _, p := range connected {
					existing[p] = struct{}{}
				}

				// Keep using these peers, and don't try new discovery/connections.
				// We don't need to search for more peers and try new connections if we already have plenty
				ctx, cancel := context.WithTimeout(ctx, collectiveDialTimeout)
			peerLoop:
				for _, id := range peersWithAddrs {
					// never dial ourselves
					if n.Host().ID() == id {
						continue
					}
					// skip peers that we are already connected to
					if _, ok := existing[id]; ok {
						continue
					}
					// skip peers that we were just connected to
					if n.Host().Network().Connectedness(id) == network.Limited {
						continue
					}
					// schedule, if there is still space to schedule (this may block)
					select {
					case connAttempts <- id:
					case <-ctx.Done():
						break peerLoop
					}
				}
				cancel()
			} else {
				// we have enough connections, slow down actively filling the peerstore
				slower()
			}
		}
	}
}

// shuffle the slice of peer IDs in-place with a RNG seeded by secure randomness.
func shufflePeers(ids peer.IDSlice) error {
	var x [8]byte // shuffling is not critical, just need to avoid basic predictability by outside peers
	if _, err := io.ReadFull(secureRand.Reader, x[:]); err != nil {
		return err
	}
	rng := rand.New(rand.NewSource(int64(binary.LittleEndian.Uint64(x[:]))))
	rng.Shuffle(len(ids), ids.Swap)
	return nil
}
