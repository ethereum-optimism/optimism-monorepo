package opnode

import (
	"context"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/services"
	"github.com/ethereum-optimism/optimism/op-node/metrics"
	rollupNode "github.com/ethereum-optimism/optimism/op-node/node"
	"github.com/ethereum-optimism/optimism/op-node/p2p"
	"github.com/ethereum-optimism/optimism/op-service/cliapp"
	"github.com/ethereum-optimism/optimism/op-service/endpoint"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

type Opnode struct {
	node *rollupNode.OpNode
}

func (o *Opnode) InteropRPC() (endpoint string, jwtSecret eth.Bytes32) {
	return o.node.InteropRPC()
}

func (o *Opnode) UserRPC() endpoint.RPC {
	return endpoint.HttpURL(o.node.HTTPEndpoint())
}

func (o *Opnode) Stop(ctx context.Context) error {
	return o.node.Stop(ctx)
}

func (o *Opnode) Stopped() bool {
	return o.node.Stopped()
}

func (o *Opnode) RuntimeConfig() rollupNode.ReadonlyRuntimeConfig {
	return o.node.RuntimeConfig()
}

func (o *Opnode) P2P() p2p.Node {
	return o.node.P2P()
}

var _ services.RollupNode = (*Opnode)(nil)

func NewOpnode(l log.Logger, c *rollupNode.Config, errFn func(error)) (*Opnode, error) {
	var cycle cliapp.Lifecycle
	c.Cancel = func(errCause error) {
		l.Warn("node requested early shutdown!", "err", errCause)
		go func() {
			postCtx, postCancel := context.WithCancel(context.Background())
			postCancel() // don't allow the stopping to continue for longer than needed
			if err := cycle.Stop(postCtx); err != nil {
				errFn(err)
			}
			l.Warn("closed op-node!")
		}()
	}
	node, err := rollupNode.New(context.Background(), c, l, "", metrics.NewMetrics(""))
	if err != nil {
		return nil, err
	}
	cycle = node
	err = node.Start(context.Background())
	if err != nil {
		return nil, err
	}
	return &Opnode{node: node}, nil
}
