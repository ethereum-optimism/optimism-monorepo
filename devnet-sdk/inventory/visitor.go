package inventory

// InventoryVisitor defines the interface for visiting inventory components
type InventoryVisitor interface {
	VisitChains(chainName string) ChainVisitor
}

type ChainVisitor interface {
	VisitNode(name string) NodeVisitor
	VisitService(name string) ServiceVisitor
}

type NodeVisitor interface {
	VisitKind(kind string)
	VisitName(name string)
	VisitSpec() NodeSpecVisitor
}

type NodeSpecVisitor interface {
	VisitKind(kind string)
	VisitEL() ServiceVisitor
	VisitCL() ServiceVisitor
}

type ServiceVisitor interface {
	VisitKind(kind string)
	VisitName(name string)
	VisitSpec() ServiceSpecVisitor
	VisitDeps(nodes []string)
}

type ServiceSpecVisitor interface {
	VisitKind(kind string)
	VisitSpec() ServiceLayerSpecVisitor
}

type ServiceLayerSpecVisitor interface {
	VisitVersion(version string)
	VisitEnv(env map[string]string)
	VisitK8s(k8s map[string]string)
}
