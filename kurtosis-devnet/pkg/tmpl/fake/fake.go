package fake

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/kurtosis-devnet/pkg/tmpl"
)

func NewFakeTemplateContext(enclave string) *tmpl.TemplateContext {
	return tmpl.NewTemplateContext(
		tmpl.WithFunction("localDockerImage", func(image string) (string, error) {
			return fmt.Sprintf("%s:%s", image, enclave), nil
		}),
		tmpl.WithFunction("localContractArtifacts", func(layer string) (string, error) {
			return fmt.Sprintf("http://host.docker.internal:0/contracts-bundle-%s.tar.gz", enclave), nil
		}),
		tmpl.WithFunction("localPrestate", func() (string, error) {
			return "http://host.docker.internal:0/proofs/op-program/cannon", nil
		}),
	)
}
