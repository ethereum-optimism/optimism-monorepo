package inventory

func (i *Inventory) Accept(visitor InventoryVisitor) {
	for _, chain := range i.Chains {
		chainVisitor := visitor.VisitChains(chain.Name)
		if chainVisitor == nil {
			continue
		}
		chain.Accept(chainVisitor)
	}
}

func (c *Chain) Accept(visitor ChainVisitor) {
	for _, node := range c.Nodes {
		nodeVisitor := visitor.VisitNode(node.Name)
		if nodeVisitor == nil {
			continue
		}
		node.Accept(nodeVisitor)
	}
	for _, service := range c.Services {
		serviceVisitor := visitor.VisitService(service.Name)
		if serviceVisitor == nil {
			continue
		}
		service.Accept(serviceVisitor)
	}
}

func (n *Node) Accept(visitor NodeVisitor) {
	visitor.VisitKind(n.Kind)
	visitor.VisitName(n.Name)
	n.Spec.Accept(visitor.VisitSpec())
}

func (ns *NodeSpec) Accept(visitor NodeSpecVisitor) {
	visitor.VisitKind(ns.Kind)
	ns.EL.Accept(visitor.VisitEL())
	ns.CL.Accept(visitor.VisitCL())
}

func (s *Service) Accept(visitor ServiceVisitor) {
	visitor.VisitKind(s.Kind)
	visitor.VisitName(s.Name)
	visitor.VisitDeps(s.Deps.Nodes)
	serviceSpecVisitor := visitor.VisitSpec()
	if serviceSpecVisitor == nil {
		return
	}
	s.Spec.Accept(serviceSpecVisitor)
}

func (ss *ServiceSpec) Accept(visitor ServiceSpecVisitor) {
	visitor.VisitKind(ss.Kind)
	serviceLayerSpecVisitor := visitor.VisitSpec()
	if serviceLayerSpecVisitor == nil {
		return
	}
	ss.Spec.Accept(serviceLayerSpecVisitor)
}

func (sls *ServiceLayerSpec) Accept(visitor ServiceLayerSpecVisitor) {
	visitor.VisitVersion(sls.Version)
	visitor.VisitEnv(sls.Env)
	visitor.VisitK8s(sls.K8s)
}
