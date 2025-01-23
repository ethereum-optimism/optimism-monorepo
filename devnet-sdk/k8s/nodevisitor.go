package k8s

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/devnet-sdk/inventory"
	"github.com/ethereum-optimism/optimism/devnet-sdk/manifest"
)

// NodeType represents the type of node (sequencer or RPC)
type NodeType string

const (
	NodeTypeSequencer NodeType = "sequencer"
	NodeTypeRPC       NodeType = "rpc"
)

// K8sVisitor visits inventory nodes for a specific chain
type K8sVisitor struct {
	params          *K8sParams
	chainVisitors   []*chainVisitor
	manifestVisitor *manifestVisitor
}

type manifestVisitor struct {
	*manifestChainVisitor
	*l2Visitor
}

func (v *K8sVisitor) BuildParams() {
	for _, chainVisitor := range v.chainVisitors {
		chainParam := ChainParams{
			ChainName: v.manifestVisitor.chainName,
			Nodes:     make([]NodeParams, 0),
		}

		for _, nodeVisitor := range chainVisitor.nodes {

			var engineType string
			switch nodeVisitor.nodeSpecVisitor.el.kind {
			case "op-geth":
				engineType = "geth"
			case "op-reth":
				engineType = "reth"
			}

			var nodeMode string
			switch nodeVisitor.nodeSpecVisitor.el.serviceSpecVisitor.kind {
			case "full":
				nodeMode = "f"
			case "archive":
				nodeMode = "a"
			}

			baseDomain := fmt.Sprintf("%s-opn-%s-%s-%s",
				chainVisitor.name,
				engineType,
				nodeMode,
				nodeVisitor.Name,
			)

			clUrl := fmt.Sprintf("https://%s-op-node.primary.infra.dev.oplabs.cloud", baseDomain)
			elUrl := fmt.Sprintf("https://%s-op-%s.primary.infra.dev.oplabs.cloud", baseDomain, engineType)
			conductorUrl := ""
			if nodeVisitor.Kind == NodeTypeSequencer {
				conductorUrl = fmt.Sprintf("https://%s-op-conductor.primary.infra.dev.oplabs.cloud", baseDomain)
			}

			nodeParam := NodeParams{
				NodeName:     nodeVisitor.Name,
				NodeType:     string(nodeVisitor.Kind),
				ClUrl:        clUrl,
				ElUrl:        elUrl,
				ConductorUrl: conductorUrl,
				ClKey:        generateHexFromString(clUrl),
				JwtSecret:    generateHexFromString(elUrl),
			}

			chainParam.Nodes = append(chainParam.Nodes, nodeParam)
		}

		v.params.Chains = append(v.params.Chains, chainParam)
	}
}

func (v *K8sVisitor) GetParams() *K8sParams {
	return v.params
}

type chainVisitor struct {
	name  string
	nodes []*nodeVisitor
}

func (v *chainVisitor) VisitNode(nodeName string) inventory.NodeVisitor {
	nodeVisitor := &nodeVisitor{
		Name: nodeName,
	}
	v.nodes = append(v.nodes, nodeVisitor)
	return nodeVisitor
}

func (v *chainVisitor) VisitService(serviceName string) inventory.ServiceVisitor {
	return &serviceVisitor{name: serviceName}
}

// nodeVisitor represents the processed node information
type nodeVisitor struct {
	*nodeSpecVisitor
	Kind NodeType
	Name string
}

func (v *nodeVisitor) VisitKind(kind string) {
	v.Kind = NodeTypeRPC
}

func (v *nodeVisitor) VisitName(name string) {
	v.Name = name
}

func (v *nodeVisitor) VisitSpec() inventory.NodeSpecVisitor {
	v.nodeSpecVisitor = &nodeSpecVisitor{}
	return v.nodeSpecVisitor
}

type nodeSpecVisitor struct {
	kind string
	el   *serviceVisitor
	cl   *serviceVisitor
}

func (v *nodeSpecVisitor) VisitKind(kind string) {
	v.kind = kind
}

func (v *nodeSpecVisitor) VisitEL() inventory.ServiceVisitor {
	v.el = &serviceVisitor{}
	return v.el
}

func (v *nodeSpecVisitor) VisitCL() inventory.ServiceVisitor {
	v.cl = &serviceVisitor{}
	return v.cl
}

type serviceVisitor struct {
	kind               string
	name               string
	deps               []string
	serviceSpecVisitor *serviceSpecVisitor
}

func (v *serviceVisitor) VisitKind(kind string) {
	v.kind = kind
}

func (v *serviceVisitor) VisitName(name string) {
	v.name = name
}

func (v *serviceVisitor) VisitDeps(nodes []string) {
	v.deps = nodes
}

func (v *serviceVisitor) VisitSpec() inventory.ServiceSpecVisitor {
	v.serviceSpecVisitor = &serviceSpecVisitor{}
	return v.serviceSpecVisitor
}

type serviceSpecVisitor struct {
	kind string
	*serviceLayerSpecVisitor
}

func (v *serviceSpecVisitor) VisitKind(kind string) {
	v.kind = kind
}

func (v *serviceSpecVisitor) VisitSpec() inventory.ServiceLayerSpecVisitor {
	v.serviceLayerSpecVisitor = &serviceLayerSpecVisitor{}
	return v.serviceLayerSpecVisitor
}

type serviceLayerSpecVisitor struct {
	version string
	env     map[string]string
	k8s     map[string]string
}

func (v *serviceLayerSpecVisitor) VisitVersion(version string) {
	v.version = version
}

func (v *serviceLayerSpecVisitor) VisitEnv(env map[string]string) {
	v.env = env
}

func (v *serviceLayerSpecVisitor) VisitK8s(k8s map[string]string) {
	v.k8s = k8s
}

// NewK8sVisitor creates a new K8sVisitor
func NewK8sVisitor() *K8sVisitor {
	return &K8sVisitor{
		params: &K8sParams{
			Cluster: DEFAULT_CLUSTER,
			Project: DEFAULT_PROJECT,
			L1Rpc:   DEFAULT_L1_RPC,
			Chains:  make([]ChainParams, 0),
		},
		chainVisitors: make([]*chainVisitor, 0),
		manifestVisitor: &manifestVisitor{
			manifestChainVisitor: &manifestChainVisitor{},
			l2Visitor:            &l2Visitor{},
		},
	}
}

func (v *K8sVisitor) VisitChains(chainName string) inventory.ChainVisitor {
	chainVisitor := &chainVisitor{
		name:  chainName,
		nodes: make([]*nodeVisitor, 0),
	}
	v.chainVisitors = append(v.chainVisitors, chainVisitor)

	return chainVisitor
}

func (v *K8sVisitor) VisitName(name string) {}

func (v *K8sVisitor) VisitType(manifestType string) {}

func (v *K8sVisitor) VisitL1() manifest.ChainVisitor {
	return nil
}

type manifestChainVisitor struct {
	chainName string
}

func (v *manifestChainVisitor) VisitName(name string) {
	v.chainName = name
}

func (v *manifestChainVisitor) VisitID(id uint64) {}

type manifestComponentVisitor struct {
	name    string
	version string
}

func (v *manifestComponentVisitor) VisitVersion(version string) {
	v.version = version
}

type l2Visitor struct {
	*manifestChainVisitor
	*manifestComponentVisitor
}

func (v *K8sVisitor) VisitL2() manifest.L2Visitor {
	chainVisitor := &manifestChainVisitor{}
	componentVisitor := &manifestComponentVisitor{}
	v.manifestVisitor.l2Visitor = &l2Visitor{manifestChainVisitor: chainVisitor, manifestComponentVisitor: componentVisitor}
	return v.manifestVisitor.l2Visitor
}

func (v *l2Visitor) VisitL2Component(name string) manifest.ComponentVisitor {
	v.manifestComponentVisitor = &manifestComponentVisitor{name: name}
	return v.manifestComponentVisitor
}

func (v *l2Visitor) VisitL2Deployment() manifest.DeploymentVisitor {
	return nil
}

func (v *l2Visitor) VisitL2Chain(int) manifest.ChainVisitor {
	chainVisitor := &manifestChainVisitor{}
	v.manifestChainVisitor = chainVisitor
	return chainVisitor
}
