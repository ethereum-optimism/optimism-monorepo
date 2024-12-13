package prestates

import (
	"net/url"

	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/trace/vm"
)

func NewPrestateSource(baseURL *url.URL, path string, localDir string, stateConverter vm.StateConverter) PrestateSource {
	if path != "" {
		return NewSinglePrestateSource(path)
	} else {
		return NewMultiPrestateProvider(baseURL, localDir, stateConverter)
	}
}
