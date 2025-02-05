package manifest

type ManifestVisitor interface {
	VisitName(name string)
	VisitType(manifestType string)
	VisitL1() ChainVisitor
	VisitL2() L2Visitor
}

type L2Visitor interface {
	VisitL2Component(name string) ComponentVisitor
	VisitL2Deployment() DeploymentVisitor
	VisitL2Chain(int) ChainVisitor
}

type ComponentVisitor interface {
	VisitVersion(version string)
}

type DeploymentVisitor interface {
	VisitDeployer() ComponentVisitor
	VisitL1Contracts() ContractsVisitor
	VisitL2Contracts() ContractsVisitor
	VisitOverride(string, interface{})
}

type ContractsVisitor interface {
	VisitVersion(version string)
	VisitLocator(locator string)
}

type ChainVisitor interface {
	VisitName(name string)
	VisitID(id uint64)
}
