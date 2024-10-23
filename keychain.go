package btcman

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/grail-rollup/btcman/indexer"
	"github.com/ledgerwatch/log/v3"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

// keychain represents an agglomeration of the keys used inside the btcman and btc indexer
type keychain struct {
	mode          BtcmanMode
	privateKeyWIF string
	privateKey    *secp256k1.PrivateKey
	publicKey     *secp256k1.PublicKey
	network       *chaincfg.Params
	logger        log.Logger
}

func NewKeychain(cfg *Config, mode BtcmanMode, network *chaincfg.Params, logger log.Logger) (Keychainer, error) {
	var privateKey *secp256k1.PrivateKey
	var publicKey *secp256k1.PublicKey

	if mode == WriterMode {
		if cfg.PrivateKey == "" {
			return nil, fmt.Errorf("private key is required for btcman in writer mode")
		}
		wif, err := btcutil.DecodeWIF(cfg.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("error decoding wif private key")
		}
		privateKey = wif.PrivKey
		publicKey = privateKey.PubKey()
	} else if mode == ReaderMode {
		if cfg.PublicKey == "" {
			return nil, fmt.Errorf("public key is required for btcman in reader mode")
		}
		publicKeyCompressedBytes, err := hex.DecodeString(cfg.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("error decoding compressed public key")
		}
		publicKey, err = secp256k1.ParsePubKey(publicKeyCompressedBytes)
		if err != nil {
			return nil, fmt.Errorf("error decoding compressed public key")
		}
	}

	return &keychain{
		mode:          mode,
		privateKeyWIF: cfg.PrivateKey,
		publicKey:     publicKey,
		privateKey:    privateKey,
		network:       network,
		logger:        logger,
	}, nil
}

// SignTransaction signs a provided unsigned transaction, indexer is used for retrieving the necessary information about previous transactions
func (k *keychain) SignTransaction(rawTransaction *wire.MsgTx, indexer indexer.Indexerer) error {
	if k.mode == ReaderMode {
		return fmt.Errorf("btcman in reader mode does not support signing transactions")
	}

	for idx, txInput := range rawTransaction.TxIn {

		prevTx, err := indexer.GetTransaction(context.Background(), txInput.PreviousOutPoint.Hash.String(), true)
		if err != nil {
			return err
		}

		subscript, err := hex.DecodeString(prevTx.Vout[txInput.PreviousOutPoint.Index].ScriptPubKey.Hex)
		if err != nil {
			return err
		}

		amount := int64(prevTx.Vout[txInput.PreviousOutPoint.Index].Value * btcutil.SatoshiPerBitcoin)

		signature, err := k.generateSignature(rawTransaction, idx, amount, subscript, indexer)
		if err != nil {
			return err
		}

		txInput.Witness = signature
	}
	k.logger.Info("Transaction signed successfully")
	return nil
}

// generateSignature is a helper for SignTransaction that generates the actual signatures
func (k *keychain) generateSignature(tx *wire.MsgTx, idx int, amt int64, subscript []byte, indexer indexer.Indexerer) (wire.TxWitness, error) {
	prevOutFetcher := NewPreviousOutPointFetcher(indexer, k.logger)

	wifKey, err := btcutil.DecodeWIF(k.privateKeyWIF)
	if err != nil {
		return nil, fmt.Errorf("failed to decode WIF: %v", err)
	}

	privKey := wifKey.PrivKey
	sigHashes := txscript.NewTxSigHashes(tx, prevOutFetcher)

	signature, err := txscript.WitnessSignature(
		tx,
		sigHashes,
		idx,
		amt,
		subscript,
		txscript.SigHashAll,
		privKey,
		true,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %v", err)
	}

	return signature, nil
}

// GetPublicKey returns the public key as string
func (k *keychain) GetPublicKey() *secp256k1.PublicKey {
	return k.publicKey
}
