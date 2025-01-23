package manifest

type ManifestAcceptor interface {
	Accept(visitor ManifestVisitor)
}

type ChainAcceptor interface {
	Accept(visitor ChainVisitor)
}

type L2Acceptor interface {
	Accept(visitor L2Visitor)
}

type DeploymentAcceptor interface {
	Accept(visitor DeploymentVisitor)
}

type ContractsAcceptor interface {
	Accept(visitor ContractsVisitor)
}

type ComponentAcceptor interface {
	Accept(visitor ComponentVisitor)
}
