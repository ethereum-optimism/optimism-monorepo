package fake

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/tmpl"
)

type PrestateInfo struct {
	URL    string            `json:"url"`
	Hashes map[string]string `json:"hashes"`
}

func NewFakeTemplateContext(enclave string) *tmpl.TemplateContext {
	return tmpl.NewTemplateContext(
		tmpl.WithFunction("localDockerImage", func(image string) (string, error) {
			return fmt.Sprintf("%s:%s", image, enclave), nil
		}),
		tmpl.WithFunction("localContractArtifacts", func(layer string) (string, error) {
			return fmt.Sprintf("http://host.docker.internal:0/contracts-bundle-%s.tar.gz", enclave), nil
		}),
		tmpl.WithFunction("localPrestate", func() (*PrestateInfo, error) {
			return &PrestateInfo{
				URL: "http://fileserver/proofs/op-program/cannon",
				Hashes: map[string]string{
					"prestate": "0x1234567890",
				},
			}, nil
		}),
	)
}
