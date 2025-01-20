package manifest

// L1Config represents L1 configuration
type L1Config struct {
	Name    string `yaml:"name"`
	ChainID uint64 `yaml:"chain_id"`
}

func (c *L1Config) Accept(visitor ChainVisitor) {
	visitor.VisitName(c.Name)
	visitor.VisitID(c.ChainID)
}

var _ ChainAcceptor = (*L1Config)(nil)

type Component struct {
	Version string `yaml:"version"`
}

func (c *Component) Accept(visitor ComponentVisitor) {
	visitor.VisitVersion(c.Version)
}

var _ ComponentAcceptor = (*Component)(nil)

type Contracts struct {
	Version string `yaml:"version"`
	Locator string `yaml:"locator"`
}

func (c *Contracts) Accept(visitor ContractsVisitor) {
	visitor.VisitLocator(c.Locator)
	visitor.VisitVersion(c.Version)
}

var _ ContractsAcceptor = (*Contracts)(nil)

// L2Deployment represents deployment configuration
type L2Deployment struct {
	OpDeployer  *Component             `yaml:"op-deployer"`
	L1Contracts *Contracts             `yaml:"l1-contracts"`
	L2Contracts *Contracts             `yaml:"l2-contracts"`
	Overrides   map[string]interface{} `yaml:"overrides"`
}

func (d *L2Deployment) Accept(visitor DeploymentVisitor) {
	d.OpDeployer.Accept(visitor.VisitDeployer())
	d.L1Contracts.Accept(visitor.VisitL1Contracts())
	d.L2Contracts.Accept(visitor.VisitL2Contracts())
	for key, value := range d.Overrides {
		visitor.VisitOverride(key, value)
	}
}

var _ DeploymentAcceptor = (*L2Deployment)(nil)

// L2Chain represents an L2 chain configuration
type L2Chain struct {
	Name    string `yaml:"name"`
	ChainID uint64 `yaml:"chain_id"`
}

func (c *L2Chain) Accept(visitor ChainVisitor) {
	visitor.VisitName(c.Name)
	visitor.VisitID(c.ChainID)
}

var _ ChainAcceptor = (*L2Chain)(nil)

// L2Config represents L2 configuration
type L2Config struct {
	Deployment *L2Deployment         `yaml:"deployment"`
	Components map[string]*Component `yaml:"components"`
	Chains     []*L2Chain            `yaml:"chains"`
}

func (c *L2Config) Accept(visitor L2Visitor) {
	for name, component := range c.Components {
		component.Accept(visitor.VisitL2Component(name))
	}
	for i, chain := range c.Chains {
		chain.Accept(visitor.VisitL2Chain(i))
	}
	c.Deployment.Accept(visitor.VisitL2Deployment())
}

var _ L2Acceptor = (*L2Config)(nil)

// Manifest represents the top-level manifest configuration
type Manifest struct {
	Name string    `yaml:"name"`
	Type string    `yaml:"type"`
	L1   *L1Config `yaml:"l1"`
	L2   *L2Config `yaml:"l2"`
}

func (m *Manifest) Accept(visitor ManifestVisitor) {
	visitor.VisitName(m.Name)
	visitor.VisitType(m.Type)
	m.L1.Accept(visitor.VisitL1())
	m.L2.Accept(visitor.VisitL2())
}

var _ ManifestAcceptor = (*Manifest)(nil)
