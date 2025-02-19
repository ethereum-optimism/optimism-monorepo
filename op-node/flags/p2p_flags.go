package flags

import (
	"fmt"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/ethereum-optimism/optimism/op-node/p2p"
	opsigner "github.com/ethereum-optimism/optimism/op-service/signer"
)

func p2pEnv(envprefix, v string) []string {
	return []string{envprefix + "_P2P_" + v}
}

var (
	DisableP2PName          = "p2p.disable"
	NoDiscoveryName         = "p2p.no-discovery"
	ScoringName             = "p2p.scoring"
	PeerScoringName         = "p2p.scoring.peers"
	PeerScoreBandsName      = "p2p.score.bands"
	BanningName             = "p2p.ban.peers"
	BanningThresholdName    = "p2p.ban.threshold"
	BanningDurationName     = "p2p.ban.duration"
	TopicScoringName        = "p2p.scoring.topics"
	P2PPrivPathName         = "p2p.priv.path"
	P2PPrivRawName          = "p2p.priv.raw"
	ListenIPName            = "p2p.listen.ip"
	ListenTCPPortName       = "p2p.listen.tcp"
	ListenUDPPortName       = "p2p.listen.udp"
	AdvertiseIPName         = "p2p.advertise.ip"
	AdvertiseTCPPortName    = "p2p.advertise.tcp"
	AdvertiseUDPPortName    = "p2p.advertise.udp"
	BootnodesName           = "p2p.bootnodes"
	StaticPeersName         = "p2p.static"
	NetRestrictName         = "p2p.netrestrict"
	HostMuxName             = "p2p.mux"
	HostSecurityName        = "p2p.security"
	PeersLoName             = "p2p.peers.lo"
	PeersHiName             = "p2p.peers.hi"
	PeersGraceName          = "p2p.peers.grace"
	NATName                 = "p2p.nat"
	UserAgentName           = "p2p.useragent"
	TimeoutNegotiationName  = "p2p.timeout.negotiation"
	TimeoutAcceptName       = "p2p.timeout.accept"
	TimeoutDialName         = "p2p.timeout.dial"
	PeerstorePathName       = "p2p.peerstore.path"
	DiscoveryPathName       = "p2p.discovery.path"
	SequencerP2PKeyName     = "p2p.sequencer.key"
	GossipMeshDName         = "p2p.gossip.mesh.d"
	GossipMeshDloName       = "p2p.gossip.mesh.lo"
	GossipMeshDhiName       = "p2p.gossip.mesh.dhi"
	GossipMeshDlazyName     = "p2p.gossip.mesh.dlazy"
	GossipFloodPublishName  = "p2p.gossip.mesh.floodpublish"
	SyncReqRespName         = "p2p.sync.req-resp"
	SyncOnlyReqToStaticName = "p2p.sync.onlyreqtostatic"
	P2PPingName             = "p2p.ping"
)

func deprecatedP2PFlags(envPrefix string) []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     PeerScoringName,
			Usage:    fmt.Sprintf("Deprecated: Use %v instead", ScoringName),
			Required: false,
			Hidden:   true,
			Category: P2PCategory,
		},
		&cli.StringFlag{
			Name:     PeerScoreBandsName,
			Usage:    "Deprecated. This option is ignored and is only present for backwards compatibility.",
			Required: false,
			Value:    "",
			Hidden:   true,
			Category: P2PCategory,
		},
		&cli.StringFlag{
			Name:     TopicScoringName,
			Usage:    fmt.Sprintf("Deprecated: Use %v instead", ScoringName),
			Required: false,
			Hidden:   true,
			Category: P2PCategory,
		},
	}
}

// None of these flags are strictly required.
// Some are hidden if they are too technical, or not recommended.
func P2PFlags(envPrefix string) []cli.Flag {
	return append([]cli.Flag{
		&cli.BoolFlag{
			Name:     DisableP2PName,
			Usage:    "Completely disable the P2P stack",
			Required: false,
			EnvVars:  p2pEnv(envPrefix, "DISABLE"),
			Category: P2PCategory,
		},
		&cli.BoolFlag{
			Name:     NoDiscoveryName,
			Usage:    "Disable Discv5 (node discovery)",
			Required: false,
			EnvVars:  p2pEnv(envPrefix, "NO_DISCOVERY"),
			Category: P2PCategory,
		},
		&cli.StringFlag{
			Name:     ScoringName,
			Usage:    "Sets the peer scoring strategy for the P2P stack. Can be one of: none or light.",
			Required: false,
			Value:    "light",
			EnvVars:  p2pEnv(envPrefix, "PEER_SCORING"),
			Category: P2PCategory,
		},
		&cli.BoolFlag{
			// Banning Flag - whether or not we want to act on the scoring
			Name:     BanningName,
			Usage:    "Enables peer banning.",
			Value:    true,
			Required: false,
			EnvVars:  p2pEnv(envPrefix, "PEER_BANNING"),
			Category: P2PCategory,
		},
		&cli.Float64Flag{
			Name:     BanningThresholdName,
			Usage:    "The minimum score below which peers are disconnected and banned.",
			Required: false,
			Value:    -100,
			EnvVars:  p2pEnv(envPrefix, "PEER_BANNING_THRESHOLD"),
			Category: P2PCategory,
		},
		&cli.DurationFlag{
			Name:     BanningDurationName,
			Usage:    "The duration that peers are banned for.",
			Required: false,
			Value:    1 * time.Hour,
			EnvVars:  p2pEnv(envPrefix, "PEER_BANNING_DURATION"),
			Category: P2PCategory,
		},
		&cli.StringFlag{
			Name: P2PPrivPathName,
			Usage: "Read the hex-encoded 32-byte private key for the peer ID from this txt file. Created if not already exists." +
				"Important to persist to keep the same network identity after restarting, maintaining the previous advertised identity.",
			Required:  false,
			Value:     "opnode_p2p_priv.txt",
			EnvVars:   p2pEnv(envPrefix, "PRIV_PATH"),
			TakesFile: true,
			Category:  P2PCategory,
		},
		&cli.StringFlag{
			// sometimes it may be ok to not persist the peer priv key as file, and instead pass it directly.
			Name:     P2PPrivRawName,
			Usage:    "The hex-encoded 32-byte private key for the peer ID",
			Required: false,
			Hidden:   true,
			Value:    "",
			EnvVars:  p2pEnv(envPrefix, "PRIV_RAW"),
			Category: P2PCategory,
		},
		&cli.StringFlag{
			Name:     ListenIPName,
			Usage:    "IP to bind LibP2P and Discv5 to",
			Required: false,
			Value:    "0.0.0.0",
			EnvVars:  p2pEnv(envPrefix, "LISTEN_IP"),
			Category: P2PCategory,
		},
		&cli.UintFlag{
			Name:     ListenTCPPortName,
			Usage:    "TCP port to bind LibP2P to. Any available system port if set to 0.",
			Required: false,
			Value:    9222,
			EnvVars:  p2pEnv(envPrefix, "LISTEN_TCP_PORT"),
			Category: P2PCategory,
		},
		&cli.UintFlag{
			Name:     ListenUDPPortName,
			Usage:    "UDP port to bind Discv5 to. Same as TCP port if left 0.",
			Required: false,
			Value:    0, // can simply match the TCP libp2p port
			EnvVars:  p2pEnv(envPrefix, "LISTEN_UDP_PORT"),
			Category: P2PCategory,
		},
		&cli.StringFlag{
			Name:     AdvertiseIPName,
			Usage:    "The IP address to advertise in Discv5, put into the ENR of the node. This may also be a hostname / domain name to resolve to an IP.",
			Required: false,
			// Ignored by default, nodes can discover their own external IP in the happy case,
			// by communicating with bootnodes. Fixed IP is recommended for faster bootstrap though.
			Value:    "",
			EnvVars:  p2pEnv(envPrefix, "ADVERTISE_IP"),
			Category: P2PCategory,
		},
		&cli.UintFlag{
			Name:     AdvertiseTCPPortName,
			Usage:    "The TCP port to advertise in Discv5, put into the ENR of the node. Set to p2p.listen.tcp value if 0.",
			Required: false,
			Value:    0,
			EnvVars:  p2pEnv(envPrefix, "ADVERTISE_TCP"),
			Category: P2PCategory,
		},
		&cli.UintFlag{
			Name:     AdvertiseUDPPortName,
			Usage:    "The UDP port to advertise in Discv5 as fallback if not determined by Discv5, put into the ENR of the node. Set to p2p.listen.udp value if 0.",
			Required: false,
			Value:    0,
			EnvVars:  p2pEnv(envPrefix, "ADVERTISE_UDP"),
			Category: P2PCategory,
		},
		&cli.StringSliceFlag{
			Name:     BootnodesName,
			Usage:    "Comma-separated base64-format ENR list. Bootnodes to start discovering other node records from.",
			Required: false,
			EnvVars:  p2pEnv(envPrefix, "BOOTNODES"),
			Category: P2PCategory,
		},
		&cli.StringFlag{
			Name: StaticPeersName,
			Usage: "Comma-separated multiaddr-format peer list. Static connections to make and maintain, these peers will be regarded as trusted. " +
				"Addresses of the local peer are ignored. Duplicate/Alternative addresses for the same peer all apply, but only a single connection per peer is maintained.",
			Required: false,
			Value:    "",
			EnvVars:  p2pEnv(envPrefix, "STATIC"),
			Category: P2PCategory,
		},
		&cli.StringFlag{
			Name:     NetRestrictName,
			Usage:    "Comma-separated list of CIDR masks. P2P will only try to connect on these networks",
			Required: false,
			EnvVars:  p2pEnv(envPrefix, "NETRESTRICT"),
			Category: P2PCategory,
		},
		&cli.StringFlag{
			Name:     HostMuxName,
			Usage:    "Comma-separated list of multiplexing protocols in order of preference. At least 1 required. Options: 'yamux','mplex'.",
			Hidden:   true,
			Required: false,
			Value:    "yamux,mplex",
			EnvVars:  p2pEnv(envPrefix, "MUX"),
			Category: P2PCategory,
		},
		&cli.StringFlag{
			Name:     HostSecurityName,
			Usage:    "Comma-separated list of transport security protocols in order of preference. At least 1 required. Options: 'noise','tls'. Set to 'none' to disable.",
			Hidden:   true,
			Required: false,
			Value:    "noise",
			EnvVars:  p2pEnv(envPrefix, "SECURITY"),
			Category: P2PCategory,
		},
		&cli.UintFlag{
			Name:     PeersLoName,
			Usage:    "Low-tide peer count. The node actively searches for new peer connections if below this amount.",
			Required: false,
			Value:    20,
			EnvVars:  p2pEnv(envPrefix, "PEERS_LO"),
			Category: P2PCategory,
		},
		&cli.UintFlag{
			Name:     PeersHiName,
			Usage:    "High-tide peer count. The node starts pruning peer connections slowly after reaching this number.",
			Required: false,
			Value:    30,
			EnvVars:  p2pEnv(envPrefix, "PEERS_HI"),
			Category: P2PCategory,
		},
		&cli.DurationFlag{
			Name:     PeersGraceName,
			Usage:    "Grace period to keep a newly connected peer around, if it is not misbehaving.",
			Required: false,
			Value:    30 * time.Second,
			EnvVars:  p2pEnv(envPrefix, "PEERS_GRACE"),
			Category: P2PCategory,
		},
		&cli.BoolFlag{
			Name:     NATName,
			Usage:    "Enable NAT traversal with PMP/UPNP devices to learn external IP.",
			Required: false,
			EnvVars:  p2pEnv(envPrefix, "NAT"),
			Category: P2PCategory,
		},
		&cli.StringFlag{
			Name:     UserAgentName,
			Usage:    "User-agent string to share via LibP2P identify. If empty it defaults to 'optimism'.",
			Hidden:   true,
			Required: false,
			Value:    "optimism",
			EnvVars:  p2pEnv(envPrefix, "AGENT"),
			Category: P2PCategory,
		},
		&cli.DurationFlag{
			Name:     TimeoutNegotiationName,
			Usage:    "Negotiation timeout, time for new peer connections to share their their supported p2p protocols",
			Hidden:   true,
			Required: false,
			Value:    10 * time.Second,
			EnvVars:  p2pEnv(envPrefix, "TIMEOUT_NEGOTIATION"),
			Category: P2PCategory,
		},
		&cli.DurationFlag{
			Name:     TimeoutAcceptName,
			Usage:    "Accept timeout, time for connection to be accepted.",
			Hidden:   true,
			Required: false,
			Value:    10 * time.Second,
			EnvVars:  p2pEnv(envPrefix, "TIMEOUT_ACCEPT"),
			Category: P2PCategory,
		},
		&cli.DurationFlag{
			Name:     TimeoutDialName,
			Usage:    "Dial timeout for outgoing connection requests",
			Hidden:   true,
			Required: false,
			Value:    10 * time.Second,
			EnvVars:  p2pEnv(envPrefix, "TIMEOUT_DIAL"),
			Category: P2PCategory,
		},
		&cli.StringFlag{
			Name: PeerstorePathName,
			Usage: "Peerstore database location. Persisted peerstores help recover peers after restarts. " +
				"Set to 'memory' to never persist the peerstore. Peerstore records will be pruned / expire as necessary. " +
				"Warning: a copy of the priv network key of the local peer will be persisted here.", // TODO: bad design of libp2p, maybe we can avoid this from happening
			Required:  false,
			TakesFile: true,
			Value:     "opnode_peerstore_db",
			EnvVars:   p2pEnv(envPrefix, "PEERSTORE_PATH"),
			Category:  P2PCategory,
		},
		&cli.StringFlag{
			Name:      DiscoveryPathName,
			Usage:     "Discovered ENRs are persisted in a database to recover from a restart without having to bootstrap the discovery process again. Set to 'memory' to never persist the peerstore.",
			Required:  false,
			TakesFile: true,
			Value:     "opnode_discovery_db",
			EnvVars:   p2pEnv(envPrefix, "DISCOVERY_PATH"),
			Category:  P2PCategory,
		},
		&cli.StringFlag{
			Name:     SequencerP2PKeyName,
			Usage:    "Hex-encoded private key for signing off on p2p application messages as sequencer.",
			Required: false,
			Value:    "",
			EnvVars:  p2pEnv(envPrefix, "SEQUENCER_KEY"),
			Category: P2PCategory,
		},
		&cli.UintFlag{
			Name:     GossipMeshDName,
			Usage:    "Configure GossipSub topic stable mesh target count, a.k.a. desired outbound degree, number of peers to gossip to",
			Required: false,
			Hidden:   true,
			Value:    p2p.DefaultMeshD,
			EnvVars:  p2pEnv(envPrefix, "GOSSIP_MESH_D"),
			Category: P2PCategory,
		},
		&cli.UintFlag{
			Name:     GossipMeshDloName,
			Usage:    "Configure GossipSub topic stable mesh low watermark, a.k.a. lower bound of outbound degree",
			Required: false,
			Hidden:   true,
			Value:    p2p.DefaultMeshDlo,
			EnvVars:  p2pEnv(envPrefix, "GOSSIP_MESH_DLO"),
			Category: P2PCategory,
		},
		&cli.UintFlag{
			Name:     GossipMeshDhiName,
			Usage:    "Configure GossipSub topic stable mesh high watermark, a.k.a. upper bound of outbound degree, additional peers will not receive gossip",
			Required: false,
			Hidden:   true,
			Value:    p2p.DefaultMeshDhi,
			EnvVars:  p2pEnv(envPrefix, "GOSSIP_MESH_DHI"),
			Category: P2PCategory,
		},
		&cli.UintFlag{
			Name:     GossipMeshDlazyName,
			Usage:    "Configure GossipSub gossip target, a.k.a. target degree for gossip only (not messaging like p2p.gossip.mesh.d, just announcements of IHAVE",
			Required: false,
			Hidden:   true,
			Value:    p2p.DefaultMeshDlazy,
			EnvVars:  p2pEnv(envPrefix, "GOSSIP_MESH_DLAZY"),
			Category: P2PCategory,
		},
		&cli.BoolFlag{
			Name:     GossipFloodPublishName,
			Usage:    "Configure GossipSub to publish messages to all known peers on the topic, outside of the mesh, also see Dlazy as less aggressive alternative.",
			Required: false,
			Hidden:   true,
			EnvVars:  p2pEnv(envPrefix, "GOSSIP_FLOOD_PUBLISH"),
			Category: P2PCategory,
		},
		&cli.BoolFlag{
			Name:     SyncReqRespName,
			Usage:    "Enables P2P req-resp alternative sync method, on both server and client side.",
			Value:    true,
			Required: false,
			EnvVars:  p2pEnv(envPrefix, "SYNC_REQ_RESP"),
			Category: P2PCategory,
		},
		&cli.BoolFlag{
			Name:     SyncOnlyReqToStaticName,
			Usage:    "Configure P2P to forward RequestL2Range requests to static peers only.",
			Value:    false,
			Required: false,
			EnvVars:  p2pEnv(envPrefix, "SYNC_ONLYREQTOSTATIC"),
			Category: P2PCategory,
		},
		&cli.BoolFlag{
			Name:     P2PPingName,
			Usage:    "Enables P2P ping-pong background service",
			Value:    true, // on by default
			Hidden:   true, // hidden, only here to disable in case of bugs.
			Required: false,
			EnvVars:  p2pEnv(envPrefix, "PING"),
		},
	}, opsigner.CLIFlags(envPrefix, P2PCategory)...)
}
