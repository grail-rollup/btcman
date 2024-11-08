package mocks

import (
	"context"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/wire"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/stretchr/testify/mock"

	"github.com/grail-rollup/btcman/indexer"
)

type Indexer struct {
	mock.Mock
}

func (m Indexer) Start(string) {}
func (m Indexer) ListUnspent(context.Context, *secp256k1.PublicKey) ([]*indexer.UTXO, error) {
	return nil, nil
}
func (m Indexer) GetHistory(context.Context, *secp256k1.PublicKey) ([]*indexer.Transaction, error) {
	return nil, nil
}
func (m Indexer) GetTransaction(context.Context, string, bool) (*btcjson.TxRawResult, error) {
	return nil, nil
}
func (m Indexer) GetBlockchainInfo(ctx context.Context) (*indexer.BlockChainInfo, error) {
	return nil, nil
}
func (m Indexer) SendTransaction(ctx context.Context, transactionHex *wire.MsgTx) (string, error) {
	return "", nil
}
func (m Indexer) GetLastInscribedTransactionsByPublicKey(ctx context.Context, publicKey *secp256k1.PublicKey, blockchainHeight int32, utxoThreshold float64) ([]*indexer.TxInfo, error) {
	return nil, nil
}
func (m Indexer) GetBlockHeader(ctx context.Context, height uint64) (string, error) {
	args := m.Called(ctx, height)
	return args.Get(0).(string), args.Error(1)
}
func (m Indexer) Disconnect() {}
