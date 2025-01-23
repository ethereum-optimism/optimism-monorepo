package inventory

type InventoryAcceptor interface {
	Accept(visitor InventoryVisitor)
}

type ChainAcceptor interface {
	Accept(visitor ChainVisitor)
}

type NodeAcceptor interface {
	Accept(visitor NodeVisitor)
}

type ServiceAcceptor interface {
	Accept(visitor ServiceVisitor)
}

type NodeSpecAcceptor interface {
	Accept(visitor NodeSpecVisitor)
}

type ServiceSpecAcceptor interface {
	Accept(visitor ServiceSpecVisitor)
}

type ServiceLayerSpecAcceptor interface {
	Accept(visitor ServiceLayerSpecVisitor)
}
