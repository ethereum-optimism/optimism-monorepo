package genesis

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/holiman/uint256"
)

// WithdrawalNetwork represents the network that withdrawals are sent to.
// Its value when marshalled in json is intended to be a consistent with its
// internal string type but is backwards-compatible with uint8 values.
// That is, WithdrawalNetwork can be unmarshalled from a JSON field into a uint8.
type WithdrawalNetwork string

// Valid returns if the withdrawal network is valid.
func (w *WithdrawalNetwork) Valid() bool {
	switch *w {
	case "local", "remote":
		return true
	default:
		return false
	}
}

// ToUint8 converts a WithdrawalNetwork to a uint8.
func (w *WithdrawalNetwork) ToUint8() uint8 {
	switch *w {
	case "remote":
		return 0
	default:
		return 1
	}
}

func (w WithdrawalNetwork) ToABI() []byte {
	out := uint256.NewInt(uint64(w.ToUint8())).Bytes32()
	return out[:]
}

// FromUint8 converts a uint8 to a WithdrawalNetwork.
func FromUint8(i uint8) WithdrawalNetwork {
	switch i {
	case 0:
		return WithdrawalNetwork("remote")
	case 1:
		return WithdrawalNetwork("local")
	default:
		return WithdrawalNetwork(strconv.Itoa(int(i)))
	}
}

// UnmarshalJSON implements the json.Unmarshaler interface, which
// allows us to ingest values of any json type as an int and run our custom conversion
func (w *WithdrawalNetwork) UnmarshalJSON(b []byte) error {
	var s WithdrawalNetwork
	if b[0] == '"' {
		if err := json.Unmarshal(b, (*string)(&s)); err != nil {
			return err
		}
	} else {
		var i uint8
		if err := json.Unmarshal(b, &i); err != nil {
			return err
		}
		s = FromUint8(i)
	}
	if !s.Valid() {
		return fmt.Errorf("invalid withdrawal network: %v", s)
	}
	*w = s
	return nil
}

func (w WithdrawalNetwork) MarshalJSON() ([]byte, error) {
	return json.Marshal(int(w.ToUint8()))
}
