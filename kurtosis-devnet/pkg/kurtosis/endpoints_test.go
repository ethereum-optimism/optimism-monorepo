package kurtosis

import (
	"testing"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/inspect"
	"github.com/stretchr/testify/assert"
)

func TestFindRPCEndpoints(t *testing.T) {
	testServices := inspect.ServiceMap{
		"el-1-geth-lighthouse": {
			"metrics":       52643,
			"tcp-discovery": 52644,
			"udp-discovery": 51936,
			"engine-rpc":    52642,
			"rpc":           52645,
			"ws":            52646,
		},
		"op-batcher-op-kurtosis": {
			"http": 53572,
		},
		"op-cl-1-op-node-op-geth-op-kurtosis": {
			"udp-discovery": 50990,
			"http":          53503,
			"tcp-discovery": 53504,
		},
		"op-el-1-op-geth-op-node-op-kurtosis": {
			"udp-discovery": 53233,
			"engine-rpc":    53399,
			"metrics":       53400,
			"rpc":           53402,
			"ws":            53403,
			"tcp-discovery": 53401,
		},
		"vc-1-geth-lighthouse": {
			"metrics": 53149,
		},
		"cl-1-lighthouse-geth": {
			"metrics":       52691,
			"tcp-discovery": 52692,
			"udp-discovery": 58275,
			"http":          52693,
		},
	}

	tests := []struct {
		name          string
		services      inspect.ServiceMap
		findFn        func(*ServiceFinder) ([]Node, EndpointMap)
		wantNodes     []Node
		wantEndpoints EndpointMap
	}{
		{
			name:     "find L1 endpoints",
			services: testServices,
			findFn: func(f *ServiceFinder) ([]Node, EndpointMap) {
				return f.FindL1Endpoints()
			},
			wantNodes: []Node{
				{
					"cl": "http://localhost:52693",
					"el": "http://localhost:52645",
				},
			},
			wantEndpoints: EndpointMap{},
		},
		{
			name:     "find op-kurtosis L2 endpoints",
			services: testServices,
			findFn: func(f *ServiceFinder) ([]Node, EndpointMap) {
				return f.FindL2Endpoints("op-kurtosis")
			},
			wantNodes: []Node{
				{
					"cl": "http://localhost:53503",
					"el": "http://localhost:53402",
				},
			},
			wantEndpoints: EndpointMap{
				"batcher": "http://localhost:53572",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			finder := NewServiceFinder(tt.services)
			gotNodes, gotEndpoints := tt.findFn(finder)
			assert.Equal(t, tt.wantNodes, gotNodes)
			assert.Equal(t, tt.wantEndpoints, gotEndpoints)
		})
	}
}
