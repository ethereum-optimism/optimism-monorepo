package nat

// SuperchainManifest represents the manifest for a devnet
// (the output of the kurtosis-devnet)
type SuperchainManifest struct {
	L1 L1Config   `json:"l1"`
	L2 []L2Config `json:"l2"`
}

type L1Config struct {
	Name      string            `json:"name"`
	Nodes     []Node            `json:"nodes"`
	Addresses L1Addresses       `json:"addresses"`
	Wallets   map[string]Wallet `json:"wallets"`
}

type L2Config struct {
	Name      string            `json:"name"`
	ID        string            `json:"id"`
	Services  L2Services        `json:"services"`
	Nodes     []Node            `json:"nodes"`
	Addresses L2Addresses       `json:"addresses"`
	Wallets   map[string]Wallet `json:"wallets"`
}

type Node struct {
	Services NodeServices `json:"services"`
}

type NodeServices struct {
	CL *Service `json:"cl,omitempty"`
	EL *Service `json:"el,omitempty"`
}

type Service struct {
	Name      string              `json:"name"`
	Endpoints map[string]Endpoint `json:"endpoints"`
}

type Endpoint struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type L2Services struct {
	Batcher  BatcherService  `json:"batcher"`
	Proposer ProposerService `json:"proposer"`
}

type BatcherService struct {
	Name      string              `json:"name"`
	Endpoints map[string]Endpoint `json:"endpoints"`
}

type ProposerService struct {
	Name      string              `json:"name"`
	Endpoints map[string]Endpoint `json:"endpoints"`
}

type L1Addresses struct {
	DelayedWETHImpl                  string `json:"delayedWETHImpl"`
	DisputeGameFactoryImpl           string `json:"disputeGameFactoryImpl"`
	L1CrossDomainMessengerImpl       string `json:"l1CrossDomainMessengerImpl"`
	L1ERC721BridgeImpl               string `json:"l1ERC721BridgeImpl"`
	L1StandardBridgeImpl             string `json:"l1StandardBridgeImpl"`
	MipsSingleton                    string `json:"mipsSingleton"`
	OPCM                             string `json:"opcm"`
	OptimismMintableERC20FactoryImpl string `json:"optimismMintableERC20FactoryImpl"`
	OptimismPortalImpl               string `json:"optimismPortalImpl"`
	PreimageOracleSingleton          string `json:"preimageOracleSingleton"`
	ProtocolVersionsImpl             string `json:"protocolVersionsImpl"`
	ProtocolVersionsProxy            string `json:"protocolVersionsProxy"`
	ProxyAdmin                       string `json:"proxyAdmin"`
	SuperchainConfigImpl             string `json:"superchainConfigImpl"`
	SuperchainConfigProxy            string `json:"superchainConfigProxy"`
	SystemConfigImpl                 string `json:"systemConfigImpl"`
}

type L2Addresses struct {
	AddressManager                     string `json:"addressManager"`
	AnchorStateRegistryImpl            string `json:"anchorStateRegistryImpl"`
	AnchorStateRegistryProxy           string `json:"anchorStateRegistryProxy"`
	DataAvailabilityChallengeImpl      string `json:"dataAvailabilityChallengeImpl"`
	DataAvailabilityChallengeProxy     string `json:"dataAvailabilityChallengeProxy"`
	DelayedWETHPermissionedGameProxy   string `json:"delayedWETHPermissionedGameProxy"`
	DelayedWETHPermissionlessGameProxy string `json:"delayedWETHPermissionlessGameProxy"`
	DisputeGameFactoryProxy            string `json:"disputeGameFactoryProxy"`
	FaultDisputeGame                   string `json:"faultDisputeGame"`
	L1CrossDomainMessengerProxy        string `json:"l1CrossDomainMessengerProxy"`
	L1ERC721BridgeProxy                string `json:"l1ERC721BridgeProxy"`
	L1StandardBridgeProxy              string `json:"l1StandardBridgeProxy"`
	OptimismMintableERC20FactoryProxy  string `json:"optimismMintableERC20FactoryProxy"`
	OptimismPortalProxy                string `json:"optimismPortalProxy"`
	PermissionedDisputeGame            string `json:"permissionedDisputeGame"`
	ProxyAdmin                         string `json:"proxyAdmin"`
	SystemConfigProxy                  string `json:"systemConfigProxy"`
}

type Wallet struct {
	Address    string `json:"address"`
	PrivateKey string `json:"private_key"`
}
