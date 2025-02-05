package kurtosis

import (
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/descriptors"
	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/kurtosis/sources/inspect"
	"github.com/stretchr/testify/assert"
)

func TestFindRPCEndpoints(t *testing.T) {
	testServices := make(inspect.ServiceMap)

	testServices["el-1-geth-lighthouse"] = inspect.PortMap{
		"metrics":       {Port: 52643},
		"tcp-discovery": {Port: 52644},
		"udp-discovery": {Port: 51936},
		"engine-rpc":    {Port: 52642},
		"rpc":           {Port: 52645},
		"ws":            {Port: 52646},
	}

	testServices["op-batcher-op-kurtosis"] = inspect.PortMap{
		"http": {Port: 53572},
	}

	testServices["op-cl-1-op-node-op-geth-op-kurtosis"] = inspect.PortMap{
		"udp-discovery": {Port: 50990},
		"http":          {Port: 53503},
		"tcp-discovery": {Port: 53504},
	}

	testServices["op-el-1-op-geth-op-node-op-kurtosis"] = inspect.PortMap{
		"udp-discovery": {Port: 53233},
		"engine-rpc":    {Port: 53399},
		"metrics":       {Port: 53400},
		"rpc":           {Port: 53402},
		"ws":            {Port: 53403},
		"tcp-discovery": {Port: 53401},
	}

	testServices["vc-1-geth-lighthouse"] = inspect.PortMap{
		"metrics": {Port: 53149},
	}

	testServices["cl-1-lighthouse-geth"] = inspect.PortMap{
		"metrics":       {Port: 52691},
		"tcp-discovery": {Port: 52692},
		"udp-discovery": {Port: 58275},
		"http":          {Port: 52693},
	}

	tests := []struct {
		name         string
		services     inspect.ServiceMap
		findFn       func(*ServiceFinder) ([]descriptors.Node, descriptors.ServiceMap)
		wantNodes    []descriptors.Node
		wantServices descriptors.ServiceMap
	}{
		{
			name:     "find L1 endpoints",
			services: testServices,
			findFn: func(f *ServiceFinder) ([]descriptors.Node, descriptors.ServiceMap) {
				return f.FindL1Services()
			},
			wantNodes: []descriptors.Node{
				{
					Services: descriptors.ServiceMap{
						"cl": descriptors.Service{
							Name: "cl-1-lighthouse-geth",
							Endpoints: descriptors.EndpointMap{
								"metrics":       {Port: 52691},
								"tcp-discovery": {Port: 52692},
								"udp-discovery": {Port: 58275},
								"http":          {Port: 52693},
							},
						},
						"el": descriptors.Service{
							Name: "el-1-geth-lighthouse",
							Endpoints: descriptors.EndpointMap{
								"metrics":       {Port: 52643},
								"tcp-discovery": {Port: 52644},
								"udp-discovery": {Port: 51936},
								"engine-rpc":    {Port: 52642},
								"rpc":           {Port: 52645},
								"ws":            {Port: 52646},
							},
						},
					},
				},
			},
			wantServices: descriptors.ServiceMap{},
		},
		{
			name:     "find op-kurtosis L2 endpoints",
			services: testServices,
			findFn: func(f *ServiceFinder) ([]descriptors.Node, descriptors.ServiceMap) {
				return f.FindL2Services("op-kurtosis")
			},
			wantNodes: []descriptors.Node{
				{
					Services: descriptors.ServiceMap{
						"cl": descriptors.Service{
							Name: "op-cl-1-op-node-op-geth-op-kurtosis",
							Endpoints: descriptors.EndpointMap{
								"udp-discovery": {Port: 50990},
								"http":          {Port: 53503},
								"tcp-discovery": {Port: 53504},
							},
						},
						"el": descriptors.Service{
							Name: "op-el-1-op-geth-op-node-op-kurtosis",
							Endpoints: descriptors.EndpointMap{
								"udp-discovery": {Port: 53233},
								"engine-rpc":    {Port: 53399},
								"metrics":       {Port: 53400},
								"tcp-discovery": {Port: 53401},
								"rpc":           {Port: 53402},
								"ws":            {Port: 53403},
							},
						},
					},
				},
			},
			wantServices: descriptors.ServiceMap{
				"batcher": descriptors.Service{
					Name: "op-batcher-op-kurtosis",
					Endpoints: descriptors.EndpointMap{
						"http": {Port: 53572},
					},
				},
			},
		},
		{
			name: "custom host in endpoint",
			services: inspect.ServiceMap{
				"op-batcher-custom-host": inspect.PortMap{
					"http": {Host: "custom.host", Port: 8080},
				},
			},
			findFn: func(f *ServiceFinder) ([]descriptors.Node, descriptors.ServiceMap) {
				return f.FindL2Services("custom-host")
			},
			wantNodes: nil,
			wantServices: descriptors.ServiceMap{
				"batcher": descriptors.Service{
					Name: "op-batcher-custom-host",
					Endpoints: descriptors.EndpointMap{
						"http": {Host: "custom.host", Port: 8080},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			finder := NewServiceFinder(tt.services)
			gotNodes, gotServices := tt.findFn(finder)
			assert.Equal(t, tt.wantNodes, gotNodes)
			assert.Equal(t, tt.wantServices, gotServices)
		})
	}
}
