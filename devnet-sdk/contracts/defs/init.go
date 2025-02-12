package defs

import (
	_ "embed"
	"encoding/json"
)

//go:generate go run gen/gen.go --src /src/integration/EventLogger.sol --contract EventLogger --out eventlogger.json

//go:embed eventlogger.json
var eventLoggerDef []byte

type ContractDef struct {
	Abi string `json:"abi"`
	Bin string `json:"bin"`
}

var EventLoggerDef ContractDef

func init() {
	json.Unmarshal(eventLoggerDef, &EventLoggerDef)
}
