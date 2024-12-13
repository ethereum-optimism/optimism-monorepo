package test

import "embed"

//go:embed configs/*json
var TestCustomChainConfigFS embed.FS
