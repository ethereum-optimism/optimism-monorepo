package cli

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/ethereum-optimism/optimism/op-node/rollup"
	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	leveldb "github.com/ipfs/go-ds-leveldb"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/multiformats/go-multiaddr"

	"github.com/ethereum-optimism/optimism/op-node/flags"
	"github.com/ethereum-optimism/optimism/op-node/p2p"

	"github.com/urfave/cli/v2"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/netutil"
)

func NewConfig(ctx *cli.Context, rollupCfg *rollup.Config) (*p2p.Config, error) {
	conf := &p2p.Config{}

	if ctx.Bool(flags.DisableP2PName) {
		conf.DisableP2P = true
		return conf, nil
	}

	p, err := loadNetworkPrivKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load p2p priv key: %w", err)
	}
	conf.Priv = p

	if err := loadListenOpts(conf, ctx); err != nil {
		return nil, fmt.Errorf("failed to load p2p listen options: %w", err)
	}

	if err := loadDiscoveryOpts(conf, ctx); err != nil {
		return nil, fmt.Errorf("failed to load p2p discovery options: %w", err)
	}

	if err := loadLibp2pOpts(conf, ctx); err != nil {
		return nil, fmt.Errorf("failed to load p2p options: %w", err)
	}

	if err := loadGossipOptions(conf, ctx); err != nil {
		return nil, fmt.Errorf("failed to load p2p gossip options: %w", err)
	}

	if err := loadScoringParams(conf, ctx, rollupCfg); err != nil {
		return nil, fmt.Errorf("failed to load p2p peer scoring options: %w", err)
	}

	if err := loadBanningOptions(conf, ctx); err != nil {
		return nil, fmt.Errorf("failed to load banning option: %w", err)
	}

	conf.EnableReqRespSync = ctx.Bool(flags.SyncReqRespName)
	conf.EnablePingService = ctx.Bool(flags.P2PPingName)
	conf.SyncOnlyReqToStatic = ctx.Bool(flags.SyncOnlyReqToStaticName)

	return conf, nil
}

func validatePort(p uint) (uint16, error) {
	if p == 0 {
		return 0, nil
	}
	if p >= (1 << 16) {
		return 0, fmt.Errorf("port out of range: %d", p)
	}
	if p < 1024 {
		return 0, fmt.Errorf("port is reserved for system: %d", p)
	}
	return uint16(p), nil
}

// loadScoringParams loads the peer scoring options from the CLI context.
func loadScoringParams(conf *p2p.Config, ctx *cli.Context, rollupCfg *rollup.Config) error {
	scoringLevel := ctx.String(flags.ScoringName)
	// Check old names for backwards compatibility
	if scoringLevel == "" {
		scoringLevel = ctx.String(flags.PeerScoringName)
	}
	if scoringLevel == "" {
		scoringLevel = ctx.String(flags.TopicScoringName)
	}
	if scoringLevel != "" {
		params, err := p2p.GetScoringParams(scoringLevel, rollupCfg)
		if err != nil {
			return err
		}
		conf.ScoringParams = params
	}

	return nil
}

// loadBanningOptions loads whether or not to ban peers from the CLI context.
func loadBanningOptions(conf *p2p.Config, ctx *cli.Context) error {
	conf.BanningEnabled = ctx.Bool(flags.BanningName)
	conf.BanningThreshold = ctx.Float64(flags.BanningThresholdName)
	conf.BanningDuration = ctx.Duration(flags.BanningDurationName)
	return nil
}

func loadListenOpts(conf *p2p.Config, ctx *cli.Context) error {
	listenIP := ctx.String(flags.ListenIPName)
	if listenIP != "" { // optional
		conf.ListenIP = net.ParseIP(listenIP)
		if conf.ListenIP == nil {
			return fmt.Errorf("failed to parse IP %q", listenIP)
		}
	}
	var err error
	conf.ListenTCPPort, err = validatePort(ctx.Uint(flags.ListenTCPPortName))
	if err != nil {
		return fmt.Errorf("bad listen TCP port: %w", err)
	}
	conf.ListenUDPPort, err = validatePort(ctx.Uint(flags.ListenUDPPortName))
	if err != nil {
		return fmt.Errorf("bad listen UDP port: %w", err)
	}
	return nil
}

func loadDiscoveryOpts(conf *p2p.Config, ctx *cli.Context) error {
	if ctx.Bool(flags.NoDiscoveryName) {
		conf.NoDiscovery = true
	}

	var err error
	conf.AdvertiseTCPPort, err = validatePort(ctx.Uint(flags.AdvertiseTCPPortName))
	if err != nil {
		return fmt.Errorf("bad advertised TCP port: %w", err)
	}
	conf.AdvertiseUDPPort, err = validatePort(ctx.Uint(flags.AdvertiseUDPPortName))
	if err != nil {
		return fmt.Errorf("bad advertised UDP port: %w", err)
	}
	adIP := ctx.String(flags.AdvertiseIPName)
	if adIP != "" { // optional
		ips, err := net.LookupIP(adIP)
		if err != nil {
			return fmt.Errorf("failed to lookup IP of %q to advertise in ENR: %w", adIP, err)
		}
		// Find the first v4 IP it resolves to
		for _, ip := range ips {
			if ipv4 := ip.To4(); ipv4 != nil {
				conf.AdvertiseIP = ipv4
				break
			}
		}
		if conf.AdvertiseIP == nil {
			return fmt.Errorf("failed to parse IP %q", adIP)
		}
	}

	dbPath := ctx.String(flags.DiscoveryPathName)
	if dbPath == "" {
		dbPath = "opnode_discovery_db"
	}
	if dbPath == "memory" {
		dbPath = ""
	}
	conf.DiscoveryDB, err = enode.OpenDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open discovery db: %w", err)
	}

	records := ctx.StringSlice(flags.BootnodesName)
	if len(records) == 0 {
		log.Info("Using default bootnodes, none provided.")
		records = p2p.DefaultBootnodes
	}

	for i, record := range records {
		record = strings.TrimSpace(record)
		if record == "" { // ignore empty records
			continue
		}

		// Resolve IP addresses of old enode URLs - geth doesn't do it any more.
		if strings.HasPrefix(record, "enode://") {
			record, err = resolveURLIP(record, net.LookupIP)
			if err != nil {
				return fmt.Errorf("resolving IP of enode URL %q: %w", record, err)
			}
		}

		nodeRecord, err := enode.Parse(enode.ValidSchemes, record)
		if err != nil {
			return fmt.Errorf("bootnode record %d (of %d) is invalid: %q err: %w", i, len(records), record, err)
		}
		conf.Bootnodes = append(conf.Bootnodes, nodeRecord)
	}

	if ctx.IsSet(flags.NetRestrictName) {
		netRestrict, err := netutil.ParseNetlist(ctx.String(flags.NetRestrictName))
		if err != nil {
			return fmt.Errorf("failed to parse net list: %w", err)
		}
		conf.NetRestrict = netRestrict
	}

	return nil
}

func resolveURLIP(rawurl string, lookupIP func(name string) ([]net.IP, error)) (string, error) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return "", fmt.Errorf("parsing URL %q: %w", rawurl, err)
	}
	ip := net.ParseIP(u.Hostname())
	if ip == nil {
		ips, err := lookupIP(u.Hostname())
		if err != nil {
			return "", fmt.Errorf("looking up IP for hostname %q: %w", u.Hostname(), err)
		}
		ip = ips[0]
	}

	// Ensure the IP is 4 bytes long for IPv4 addresses.
	if ipv4 := ip.To4(); ipv4 != nil {
		ip = ipv4
	}

	// reassemble
	port := u.Port()
	u.Host = ip.String()
	if port != "" {
		u.Host += ":" + port
	}
	return u.String(), nil
}

func loadLibp2pOpts(conf *p2p.Config, ctx *cli.Context) error {
	addrs := strings.Split(ctx.String(flags.StaticPeersName), ",")
	for i, addr := range addrs {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue // skip empty multi addrs
		}
		a, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			return fmt.Errorf("failed to parse multi addr of static peer %d (out of %d): %q err: %w", i, len(addrs), addr, err)
		}
		conf.StaticPeers = append(conf.StaticPeers, a)
	}

	for _, v := range strings.Split(ctx.String(flags.HostMuxName), ",") {
		v = strings.ToLower(strings.TrimSpace(v))
		switch v {
		case "yamux":
			conf.HostMux = append(conf.HostMux, p2p.YamuxC())
		case "mplex":
			conf.HostMux = append(conf.HostMux, p2p.MplexC())
		default:
			return fmt.Errorf("could not recognize mux %s", v)
		}
	}

	secArr := strings.Split(ctx.String(flags.HostSecurityName), ",")
	for _, v := range secArr {
		v = strings.ToLower(strings.TrimSpace(v))
		switch v {
		case "none": // no security, for debugging etc.
			if len(conf.HostSecurity) > 0 || len(secArr) > 1 {
				return errors.New("cannot mix secure transport protocols with no-security")
			}
			conf.NoTransportSecurity = true
		case "noise":
			conf.HostSecurity = append(conf.HostSecurity, p2p.NoiseC())
		case "tls":
			conf.HostSecurity = append(conf.HostSecurity, p2p.TlsC())
		default:
			return fmt.Errorf("could not recognize security %s", v)
		}
	}

	conf.PeersLo = ctx.Uint(flags.PeersLoName)
	conf.PeersHi = ctx.Uint(flags.PeersHiName)
	conf.PeersGrace = ctx.Duration(flags.PeersGraceName)
	conf.NAT = ctx.Bool(flags.NATName)
	conf.UserAgent = ctx.String(flags.UserAgentName)
	conf.TimeoutNegotiation = ctx.Duration(flags.TimeoutNegotiationName)
	conf.TimeoutAccept = ctx.Duration(flags.TimeoutAcceptName)
	conf.TimeoutDial = ctx.Duration(flags.TimeoutDialName)

	peerstorePath := ctx.String(flags.PeerstorePathName)
	if peerstorePath == "" {
		return errors.New("peerstore path must be specified, use 'memory' to explicitly not persist peer records")
	}

	var err error
	var store ds.Batching
	if peerstorePath == "memory" {
		store = sync.MutexWrap(ds.NewMapDatastore())
	} else {
		store, err = leveldb.NewDatastore(peerstorePath, nil) // default leveldb options are fine
		if err != nil {
			return fmt.Errorf("failed to open leveldb db for peerstore: %w", err)
		}
	}
	conf.Store = store

	return nil
}

func loadNetworkPrivKey(ctx *cli.Context) (*crypto.Secp256k1PrivateKey, error) {
	raw := ctx.String(flags.P2PPrivRawName)
	if raw != "" {
		return parsePriv(raw)
	}
	keyPath := ctx.String(flags.P2PPrivPathName)
	if keyPath == "" {
		return nil, errors.New("no p2p private key path specified, cannot auto-generate key without path")
	}
	f, err := os.OpenFile(keyPath, os.O_RDONLY, 0o600)
	if os.IsNotExist(err) {
		p, _, err := crypto.GenerateSecp256k1Key(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("failed to generate new p2p priv key: %w", err)
		}
		b, err := p.Raw()
		if err != nil {
			return nil, fmt.Errorf("failed to encode new p2p priv key: %w", err)
		}
		f, err := os.OpenFile(keyPath, os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, fmt.Errorf("failed to store new p2p priv key: %w", err)
		}
		defer f.Close()
		if _, err := f.WriteString(hex.EncodeToString(b)); err != nil {
			return nil, fmt.Errorf("failed to write new p2p priv key: %w", err)
		}
		return (p).(*crypto.Secp256k1PrivateKey), nil
	} else {
		defer f.Close()
		data, err := io.ReadAll(f)
		if err != nil {
			return nil, fmt.Errorf("failed to read priv key file: %w", err)
		}
		return parsePriv(strings.TrimSpace(string(data)))
	}
}

func parsePriv(data string) (*crypto.Secp256k1PrivateKey, error) {
	if len(data) > 2 && data[:2] == "0x" {
		data = data[2:]
	}
	b, err := hex.DecodeString(data)
	if err != nil {
		return nil, errors.New("p2p priv key is not formatted in hex chars")
	}
	p, err := crypto.UnmarshalSecp256k1PrivateKey(b)
	if err != nil {
		// avoid logging the priv key in the error, but hint at likely input length problem
		return nil, fmt.Errorf("failed to parse priv key from %d bytes", len(b))
	}
	return (p).(*crypto.Secp256k1PrivateKey), nil
}

func loadGossipOptions(conf *p2p.Config, ctx *cli.Context) error {
	conf.MeshD = ctx.Int(flags.GossipMeshDName)
	conf.MeshDLo = ctx.Int(flags.GossipMeshDloName)
	conf.MeshDHi = ctx.Int(flags.GossipMeshDhiName)
	conf.MeshDLazy = ctx.Int(flags.GossipMeshDlazyName)
	conf.FloodPublish = ctx.Bool(flags.GossipFloodPublishName)
	return nil
}
