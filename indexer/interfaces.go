package indexer

import (
	"context"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/wire"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

type Indexerer interface {
	Start(string)
	ListUnspent(context.Context, *secp256k1.PublicKey) ([]*UTXO, error)
	GetHistory(context.Context, *secp256k1.PublicKey) ([]*Transaction, error)
	GetTransaction(context.Context, string, bool) (*btcjson.TxRawResult, error)
	GetBlockchainInfo(ctx context.Context) (*BlockChainInfo, error)
	SendTransaction(ctx context.Context, transactionHex *wire.MsgTx) (string, error)
	GetLastInscribedTransactionsByPublicKey(ctx context.Context, publicKey *secp256k1.PublicKey, blockchainHeight int32, utxoThreshold float64) ([]*TxInfo, error)
	Disconnect()
}
