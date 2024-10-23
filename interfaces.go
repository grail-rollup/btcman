package btcman

import (
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/wire"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/grail-rollup/btcman/indexer"
)

// Clienter is the interface for creating inscriptions in a btc transaction
type Clienter interface {
	GetBlockchainHeight() (int32, error)
	ListUnspent() ([]*indexer.UTXO, error)
	GetHistory() ([]*indexer.Transaction, error)
	GetTransaction(txid string, verbose bool) (*btcjson.TxRawResult, error)
	Shutdown()
}

type Keychainer interface {
	SignTransaction(rawTransaction *wire.MsgTx, indexer indexer.Indexerer) error
	GetPublicKey() *secp256k1.PublicKey
}
