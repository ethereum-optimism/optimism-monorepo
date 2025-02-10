package env

import (
	"bytes"
	"fmt"
	"html/template"
	"net/url"
	"path/filepath"

	"github.com/ethereum-optimism/optimism/devnet-sdk/descriptors"
)

const (
	EnvURLVar    = "DEVNET_ENV_URL"
	ChainNameVar = "DEVNET_CHAIN_NAME"
)

type ChainConfig struct {
	chain     *descriptors.Chain
	devnetURL string
	name      string
}

type ChainEnv struct {
	Motd    string
	EnvVars map[string]string
}

func (c *ChainConfig) getRpcUrl() (string, error) {
	if len(c.chain.Nodes) == 0 {
		return "", fmt.Errorf("chain '%s' has no nodes", c.chain.Name)
	}

	// Get RPC endpoint from the first node's execution layer service
	elService, ok := c.chain.Nodes[0].Services["el"]
	if !ok {
		return "", fmt.Errorf("no execution layer service found for chain '%s'", c.chain.Name)
	}

	rpcEndpoint, ok := elService.Endpoints["rpc"]
	if !ok {
		return "", fmt.Errorf("no RPC endpoint found for chain '%s'", c.chain.Name)
	}

	return fmt.Sprintf("http://%s:%d", rpcEndpoint.Host, rpcEndpoint.Port), nil
}

func (c *ChainConfig) getJwtSecret() (string, error) {
	jwt := c.chain.JWT
	if len(jwt) >= 2 && jwt[:2] == "0x" {
		jwt = jwt[2:]
	}

	return jwt, nil
}

func (c *ChainConfig) motd() string {
	tmpl := `You're in a {{.Name}} chain subshell.

	Some addresses of interest:
	{{ range $key, $value := .Addresses -}}
		{{ printf "%-35s" $key }} = {{ $value }}
	{{ end -}}
	`

	t := template.Must(template.New("motd").Parse(tmpl))

	var buf bytes.Buffer
	if err := t.Execute(&buf, c.chain); err != nil {
		panic(err)
	}

	return buf.String()
}

func (c *ChainConfig) GetEnv() (*ChainEnv, error) {
	mapping := map[string]func() (string, error){
		"ETH_RPC_URL":        c.getRpcUrl,
		"ETH_RPC_JWT_SECRET": c.getJwtSecret,
	}

	motd := c.motd()
	envVars := make(map[string]string)
	for key, fn := range mapping {
		value, err := fn()
		if err != nil {
			return nil, err
		}
		envVars[key] = value
	}

	// To allow commands within the shell to know which devnet and chain they are in
	absPath := c.devnetURL
	if u, err := url.Parse(c.devnetURL); err == nil {
		if u.Scheme == "" || u.Scheme == "file" {
			// make sure the path is absolute
			if abs, err := filepath.Abs(u.Path); err == nil {
				absPath = abs
			}
		}
	}
	envVars[EnvURLVar] = absPath
	envVars[ChainNameVar] = c.name

	return &ChainEnv{
		Motd:    motd,
		EnvVars: envVars,
	}, nil
}
