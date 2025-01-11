package nat

type Config struct {
	RPCURL             string
	SenderSecretKey    string `json:"-"`
	ReceiverPublicKeys []string

	Validators []Validator
}

func NewConfig(rpcURL string, senderSecretKey string, receiverPublicKeys []string, validators []Validator) *Config {
	return &Config{
		RPCURL:             rpcURL,
		SenderSecretKey:    senderSecretKey,
		ReceiverPublicKeys: receiverPublicKeys,
		Validators:         validators,
	}
}
