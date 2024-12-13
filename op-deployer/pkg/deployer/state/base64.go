package state

import (
	"encoding/base64"
	"encoding/json"
)

type Base64Bytes []byte

func (b Base64Bytes) MarshalJSON() ([]byte, error) {
	if len(b) == 0 {
		return []byte(`null`), nil
	}

	encoded := base64.StdEncoding.EncodeToString(b)
	return []byte(`"` + encoded + `"`), nil
}

func (b *Base64Bytes) UnmarshalJSON(data []byte) error {
	var dataStr string
	if err := json.Unmarshal(data, &dataStr); err != nil {
		return err
	}
	decoded, err := base64.StdEncoding.DecodeString(dataStr)
	if err != nil {
		return err
	}
	*b = decoded
	return nil
}
