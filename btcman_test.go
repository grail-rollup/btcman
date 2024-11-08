package btcman

import (
	"fmt"
	"testing"

	"github.com/grail-rollup/btcman/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

func TestGetBlockHeader(t *testing.T) {
	mockIndexer := new(mocks.Indexer)
	config := Config{
		Mode:        string(ReaderMode),
		Net:         "regtest",
		PublicKey:   "03e392587e5c9fdb0b4f96614d8a557a953e6cb1253298a60ff947e3193adedbb7",
		IndexerHost: "localhost",
		IndexerPort: "0000",
	}

	btcman := &Client{
		logger:                   nil,
		keychain:                 nil,
		cfg:                      config,
		netParams:                nil,
		address:                  nil,
		IndexerClient:            mockIndexer,
		consolidationStopChannel: nil,
		utxoThreshold:            0,
		isDebug:                  false,
	}

	hex1 := "00000020c1bf17d70dfd2b25df6d0bd40a2bb46bbedb51faa3a3233c4189645deb1ed45ff410088ee8cb8847f309b8c81e9ce7f87b9a9024bb429ccd531b6e30f7cd707f5d642b67ffff7f2000000000"
	hex2 := "00000020737c079ed6ebe84e014df4896cc381ad3436d7fdf933fd113dbe6f78fe14654f5500aa66df88ceeee76c0d2219222b467a20faf8fd1e6aa8661678b0accc2e915d642b67ffff7f2002000000"
	mockIndexer.On("GetBlockHeader", mock.Anything, uint64(1)).Return(hex1, nil)
	mockIndexer.On("GetBlockHeader", mock.Anything, uint64(2)).Return(hex2, nil)

	bh1, _ := btcman.GetBlockHeader(1)
	bh2, _ := btcman.GetBlockHeader(2)

	assert.Equal(t, bh2.PrevBlock, bh1.BlockHash())
}
