package v2_0_0

import (
	"encoding/json"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/opcm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/lmittmann/w3"
)

type UpgradeOPChainInput struct {
	Prank               common.Address  `json:"prank"`
	Opcm                common.Address  `json:"opcm"`
	EncodedChainConfigs []OPChainConfig `evm:"-" json:"chainConfigs"`
}

type OPChainConfig struct {
	SystemConfigProxy common.Address `json:"systemConfigProxy"`
	ProxyAdmin        common.Address `json:"proxyAdmin"`
	AbsolutePrestate  common.Hash    `json:"absolutePrestate"`
}

var opChainConfigEncoder = w3.MustNewFunc("dummy((address systemConfigProxy,address proxyAdmin,bytes32 absolutePrestate)[])", "")

func (u *UpgradeOPChainInput) OpChainConfigs() ([]byte, error) {
	data, err := opChainConfigEncoder.EncodeArgs(u.EncodedChainConfigs)
	if err != nil {
		return nil, fmt.Errorf("failed to encode chain configs: %w", err)
	}
	return data[4:], nil
}

type UpgradeOPChain struct {
	Run func(input common.Address)
}

func Upgrade(host *script.Host, input UpgradeOPChainInput) error {
	return opcm.RunScriptVoid[UpgradeOPChainInput](host, input, "UpgradeOPChain.s.sol", "UpgradeOPChain")
}

type Upgrader struct{}

func (u *Upgrader) Upgrade(host *script.Host, input json.RawMessage) error {
	var upgradeInput UpgradeOPChainInput
	if err := json.Unmarshal(input, &upgradeInput); err != nil {
		return fmt.Errorf("failed to unmarshal input: %w", err)
	}
	return Upgrade(host, upgradeInput)
}

func (u *Upgrader) SupportsVersion(version string) bool {
	return version == "2.0.0"
}

func (u *Upgrader) ArtifactsURL() string {
	return ""
}

var DefaultUpgrader = new(Upgrader)
