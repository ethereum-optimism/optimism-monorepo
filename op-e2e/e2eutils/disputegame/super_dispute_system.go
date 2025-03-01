package disputegame

import (
	"strings"
	"time"

	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/challenger"
	"github.com/ethereum-optimism/optimism/op-e2e/interop"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/endpoint"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/ethclient"
)

type SuperDisputeSystem struct {
	sys interop.SuperSystem
}

func (s *SuperDisputeSystem) SupervisorClient() *sources.SupervisorClient {
	return s.sys.SupervisorClient()
}

func NewSuperDisputeSystem(sys interop.SuperSystem) *SuperDisputeSystem {
	return &SuperDisputeSystem{sys}
}

func splitName(name string) (string, string) {
	parts := strings.SplitN(name, "/", 2)
	if len(parts) != 2 {
		panic("Invalid super system name: " + name)
	}
	return parts[0], parts[1]
}

func (s *SuperDisputeSystem) L1BeaconEndpoint() endpoint.RestHTTP {
	beacon := s.sys.L1Beacon()
	return endpoint.RestHTTPURL(beacon.BeaconAddr())
}

func (s *SuperDisputeSystem) NodeEndpoint(name string) endpoint.RPC {
	if name == "l1" {
		return s.sys.L1().UserRPC()
	}
	network, node := splitName(name)
	return s.sys.L2GethEndpoint(network, node)
}

func (s *SuperDisputeSystem) NodeClient(name string) *ethclient.Client {
	if name == "l1" {
		return s.sys.L1GethClient()
	}
	network, node := splitName(name)
	return s.sys.L2GethClient(network, node)
}

func (s *SuperDisputeSystem) RollupEndpoint(name string) endpoint.RPC {
	network, node := splitName(name)
	return s.sys.L2RollupEndpoint(network, node)
}

func (s *SuperDisputeSystem) RollupClient(name string) *sources.RollupClient {
	network, node := splitName(name)
	return s.sys.L2RollupClient(network, node)
}

func (s *SuperDisputeSystem) DisputeGameFactoryAddr() common.Address {
	return s.sys.DisputeGameFactoryAddr()
}

func (s *SuperDisputeSystem) RollupCfgs() []*rollup.Config {
	networks := s.sys.L2IDs()
	cfgs := make([]*rollup.Config, len(networks))
	for i, network := range networks {
		cfgs[i] = s.sys.RollupConfig(network)
	}
	return cfgs
}

func (s *SuperDisputeSystem) L2Genesises() []*core.Genesis {
	networks := s.sys.L2IDs()
	cfgs := make([]*core.Genesis, len(networks))
	for i, network := range networks {
		cfgs[i] = s.sys.L2Genesis(network)
	}
	return cfgs
}

func (s *SuperDisputeSystem) PrestateVariant() challenger.PrestateVariant {
	return challenger.InteropVariant
}

func (s *SuperDisputeSystem) AdvanceTime(duration time.Duration) {
	s.sys.AdvanceL1Time(duration)
}

var _ DisputeSystem = (*SuperDisputeSystem)(nil)
