package btcman

import (
	"errors"

	"github.com/btcsuite/btcd/chaincfg"
)

func loadNetwork(networkInput string) (*chaincfg.Params, error) {
	switch networkInput {
	case "mainnet":
		return &chaincfg.MainNetParams, nil
	case "testnet":
		return &chaincfg.TestNet3Params, nil
	case "regtest":
		return &chaincfg.RegressionNetParams, nil
	default:
		return nil, errors.New("invalid network")
	}
}

func loadMode(modeInput string) (BtcmanMode, error) {
	switch modeInput {
	case "reader":
		return ReaderMode, nil
	case "writer":
		return WriterMode, nil
	default:
		return InvalidMode, errors.New("invalid mode")
	}
}

func loadConsolidationValues(cfg *Config) (consolidationInterval, consolidationTransactionFee, utxoThreshold, minUtxoConsolidationAmount int) {
	if consolidationInterval = cfg.ConsolidationInterval; consolidationInterval == 0 {
		consolidationInterval = DEFAULT_CONSOLIDATION_INTERVAL
	}
	if consolidationTransactionFee = cfg.ConsolidationTransactionFee; consolidationTransactionFee == 0 {
		consolidationTransactionFee = DEFAULT_CONSOLIDATION_TRANSACTION_FEE
	}
	if utxoThreshold = cfg.UtxoThreshold; utxoThreshold == 0 {
		utxoThreshold = DEFAULT_UTXO_THRESHOLD
	}
	if minUtxoConsolidationAmount = cfg.MinUtxoConsolidationAmount; minUtxoConsolidationAmount == 0 {
		minUtxoConsolidationAmount = DEFAULT_MIN_UTXO_CONSOLIDATION_AMOUNT
	}

	return consolidationInterval, consolidationTransactionFee, utxoThreshold, minUtxoConsolidationAmount
}
