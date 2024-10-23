package btcman

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name          string
		config        Config
		expectedError error
	}{
		{
			name:          "Empty config",
			config:        Config{},
			expectedError: fmt.Errorf("invalid config"),
		},
		{
			name: "Reader mode config",
			config: Config{
				Mode:        string(ReaderMode),
				Net:         "regtest",
				PublicKey:   "03e392587e5c9fdb0b4f96614d8a557a953e6cb1253298a60ff947e3193adedbb7",
				IndexerHost: "localhost",
				IndexerPort: "0000",
			},
			expectedError: nil,
		},
		{
			name: "Reader mode without public key config",
			config: Config{
				Mode:        string(ReaderMode),
				Net:         "regtest",
				PrivateKey:  "cSaejkcWwU25jMweWEewRSsrVQq2FGTij1xjXv4x1XvxVRF1ZCr3",
				IndexerHost: "localhost",
				IndexerPort: "0000",
			},
			expectedError: fmt.Errorf("public key is required for btcman in reader mode"),
		},
		{
			name: "Writer mode config",
			config: Config{
				Mode:        string(WriterMode),
				Net:         "regtest",
				PrivateKey:  "cSaejkcWwU25jMweWEewRSsrVQq2FGTij1xjXv4x1XvxVRF1ZCr3",
				IndexerHost: "localhost",
				IndexerPort: "0000",
			},
			expectedError: nil,
		},
		{
			name: "Writer mode without private key config",
			config: Config{
				Mode:        string(WriterMode),
				Net:         "regtest",
				PublicKey:   "03e392587e5c9fdb0b4f96614d8a557a953e6cb1253298a60ff947e3193adedbb7",
				IndexerHost: "localhost",
				IndexerPort: "0000",
			},
			expectedError: fmt.Errorf("private key is required for btcman in writer mode"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClient(tt.config)
			assert.Equal(t, tt.expectedError, err)
		})
	}
}
