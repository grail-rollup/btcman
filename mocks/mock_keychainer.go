package mocks

import (
	"github.com/btcsuite/btcd/wire"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/grail-rollup/btcman/indexer"
	"github.com/stretchr/testify/mock"
)

type Keychainer struct {
	mock.Mock
}

func (m *Keychainer) SignTransaction(rawTransaction *wire.MsgTx, indexer indexer.Indexerer) error {
	return nil
}

func (m *Keychainer) GetPublicKey() *secp256k1.PublicKey {
	return nil
}
