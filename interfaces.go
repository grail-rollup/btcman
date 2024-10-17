package btcman

import (
	"github.com/btcsuite/btcd/wire"
	"github.com/grail-rollup/btcman/indexer"
)

// Clienter is the interface for creating inscriptions in a btc transaction
type Clienter interface {
	Inscribe(data []byte) error
	DecodeInscription() (string, error)
	Shutdown()
}

type Keychainer interface {
	SignTransaction(rawTransaction *wire.MsgTx, indexer indexer.Indexerer) error
	GetPublicKey() string
}
