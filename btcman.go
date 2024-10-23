package btcman

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/grail-rollup/btcman/indexer"
	"github.com/ledgerwatch/log/v3"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

// Client is the btc client that interacts with th btc chain
type Client struct {
	keychain                 Keychainer
	netParams                *chaincfg.Params
	cfg                      Config
	address                  *btcutil.Address
	IndexerClient            indexer.Indexerer
	consolidationStopChannel chan struct{}
}

const (
	DEFAULT_CONSOLIDATION_INTERVAL        = 60
	DEFAULT_CONSOLIDATION_TRANSACTION_FEE = 1000
	DEFAULT_UTXO_THRESHOLD                = 5000
	DEFAULT_MIN_UTXO_CONSOLIDATION_AMOUNT = 10
)

func NewClient(cfg Config) (Clienter, error) {
	log.Debug("Creating btcman")

	// Set default config values
	if cfg.ConsolidationInterval == 0 {
		cfg.ConsolidationInterval = DEFAULT_CONSOLIDATION_INTERVAL
	}
	if cfg.ConsolidationTransactionFee == 0 {
		cfg.ConsolidationTransactionFee = DEFAULT_CONSOLIDATION_TRANSACTION_FEE
	}
	if cfg.UtxoThreshold == 0 {
		cfg.UtxoThreshold = DEFAULT_UTXO_THRESHOLD
	}
	if cfg.MinUtxoConsolidationAmount == 0 {
		cfg.MinUtxoConsolidationAmount = DEFAULT_MIN_UTXO_CONSOLIDATION_AMOUNT
	}

	isValid := IsValidBtcConfig(&cfg)
	if !isValid {
		err := errors.New("invalid config")
		return nil, err
	}

	// Check if the network is valid
	var network chaincfg.Params
	switch cfg.Net {
	case "mainnet":
		network = chaincfg.MainNetParams
	case "testnet":
		network = chaincfg.TestNet3Params
	case "regtest":
		network = chaincfg.RegressionNetParams
	default:
		err := errors.New("invalid network")
		return nil, err
	}

	var mode BtcmanMode
	switch cfg.Mode {
	case "reader":
		mode = ReaderMode
	case "writer":
		mode = WriterMode
	}

	keychain, err := NewKeychain(&cfg, mode, &network)
	if err != nil {
		return nil, err
	}
	address, err := indexer.PublicKeyToAddress(keychain.GetPublicKey(), &network)

	indexer := indexer.NewIndexer(cfg.EnableIndexerDebug)
	indexer.Start(fmt.Sprintf("%s:%s", cfg.IndexerHost, cfg.IndexerPort))

	consolidateTxFee := float64(cfg.ConsolidationTransactionFee)

	stopChannel := make(chan struct{})

	btcman := Client{
		keychain:                 keychain,
		cfg:                      cfg,
		netParams:                &network,
		address:                  &address,
		IndexerClient:            indexer,
		consolidationStopChannel: stopChannel,
	}

	if mode == WriterMode {
		consolidationInterval := time.Second * time.Duration(cfg.ConsolidationInterval)
		ticker := time.NewTicker(consolidationInterval)

		go func() {
			for {
				select {
				case <-btcman.consolidationStopChannel:
					ticker.Stop()
					return
				case <-ticker.C:
					log.Debug("Trying to consolidate")
					utxos, err := btcman.ListUnspent()
					if err != nil {
						log.Error("Failed to list utxos", "err", err)
					}

					btcman.consolidateUTXOS(utxos, consolidateTxFee, cfg.MinUtxoConsolidationAmount)
				}
			}
		}()
	}

	return &btcman, nil
}

// Shutdown closes the RPC client
func (client *Client) Shutdown() {
	close(client.consolidationStopChannel)
	client.IndexerClient.Disconnect()
}

// getUTXO returns a UTXO spendable by address, consolidates the address utxo set if needed
func (client *Client) getUTXO() (*indexer.UTXO, error) {
	utxos, err := client.ListUnspent()
	if err != nil {
		return nil, err
	}
	if len(utxos) == 0 {
		return nil, fmt.Errorf("there are no UTXOs")
	}

	utxoIndex := client.getIndexOfUtxoAboveThreshold(float64(client.cfg.UtxoThreshold), utxos)
	if utxoIndex == -1 {
		return nil, fmt.Errorf("can't find utxo to inscribe")
	}

	utxo := utxos[utxoIndex]

	log.Info("UTXO for address was found")
	return utxo, nil
}

// consolidateUTXOS combines multiple utxo in one if the utxos are under a specific threshold and over a specific count
func (client *Client) consolidateUTXOS(utxos []*indexer.UTXO, consolidationFee float64, minUtxoCountConsolidate int) {
	if len(utxos) == 0 {
		log.Info("Address has zero utxos.. skipping consolidation")
		return
	}

	var inputs []btcjson.TransactionInput
	dustAmount := btcutil.Amount(546)
	totalAmount := btcutil.Amount(0)

	for _, utxo := range utxos {
		amount := btcutil.Amount(utxo.Value)
		thresholdAmount := btcutil.Amount(float64(client.cfg.UtxoThreshold))
		if amount < thresholdAmount && amount > dustAmount {
			inputs = append(inputs, btcjson.TransactionInput{
				Txid: utxo.TxHash,
				Vout: uint32(utxo.TxPos),
			})
			log.Debug("Adding utxo", "hash", utxo.TxHash, "amount", amount)
			totalAmount += amount
		}
	}

	if len(inputs) < minUtxoCountConsolidate || totalAmount <= btcutil.Amount(consolidationFee) {
		log.Info("Not enough UTXOs under the specified amount to consolidate.", "utxos",
			len(inputs), "minUtxoCount", minUtxoCountConsolidate, "utxoThreshold", float64(client.cfg.UtxoThreshold))
		return
	}

	log.Info("Consolidating utxos", "utxos", len(inputs), "amount", totalAmount)

	outputAmount := totalAmount - btcutil.Amount(consolidationFee*(float64(len(inputs))*0.1))

	rawTx, err := client.createRawTransaction(inputs, &outputAmount, client.address)
	if err != nil {
		log.Error("error creating raw transaction", "err", err)
		return
	}

	err = client.keychain.SignTransaction(rawTx, client.IndexerClient)
	if err != nil {
		log.Error("error signing raw transaction", "err", err)
		return
	}

	txHash, err := client.IndexerClient.SendTransaction(context.Background(), rawTx)
	if err != nil {
		log.Error("error sending transaction", "err", err)
		return
	}
	log.Info("UTXOs consolidated successfully", "txHash", txHash)
}

// getUtxoAboveThreshold returns the index of utxo over a specific threshold from a utxo set, if doesn't exist returns -1
func (client *Client) getIndexOfUtxoAboveThreshold(threshold float64, utxos []*indexer.UTXO) int {
	for index, utxo := range utxos {
		if float64(utxo.Value) >= threshold {
			return index
		}
	}
	return -1
}

// createInscriptionRequest cretes the request for the insription with the inscription data
func (client *Client) createInscriptionRequest(data []byte) (*InscriptionRequest, error) {
	utxo, err := client.getUTXO()
	if err != nil {
		return nil, err
	}

	commitTxOutPoint := new(wire.OutPoint)
	inTxid, err := chainhash.NewHashFromStr(utxo.TxHash)
	if err != nil {
		log.Error("Failed to create inscription request")
		return nil, err
	}

	commitTxOutPoint = wire.NewOutPoint(inTxid, uint32(utxo.TxPos))

	dataList := make([]InscriptionData, 0)

	dataList = append(dataList, InscriptionData{
		ContentType: "application/octet-stream",
		Body:        data,
		Destination: (*client.address).String(),
	})

	request := InscriptionRequest{
		CommitTxOutPointList: []*wire.OutPoint{commitTxOutPoint},
		CommitFeeRate:        3,
		FeeRate:              2,
		DataList:             dataList,
		SingleRevealTxOnly:   true,
		// RevealOutValue:       500,
	}
	return &request, nil
}

// createInscriptionTool returns a new inscription tool struct
func (client *Client) createInscriptionTool(message []byte) (*InscriptionTool, error) {
	request, err := client.createInscriptionRequest(message)
	if err != nil {
		return nil, err
	}

	tool, err := NewInscriptionTool(client.netParams, request, client.IndexerClient, client.keychain)
	if err != nil {
		return nil, err
	}
	return tool, nil
}

// Inscribe creates an inscription of data into a btc transaction
func (client *Client) Inscribe(data []byte) error {
	tool, err := client.createInscriptionTool(data)
	if err != nil {
		return err
	}

	commitTxHash, revealTxHashList, inscriptions, fees, err := tool.Inscribe()
	if err != nil {
		return err
	}
	revealTxHash := revealTxHashList[0]
	inscription := inscriptions[0]

	log.Debug("CommitTxHash: %s", commitTxHash.String())
	log.Debug("RevealTxHash: %s", revealTxHash.String())
	log.Debug("Inscription: %s", inscription)
	log.Debug("Successful inscription", "commitTx", commitTxHash.String(),
		"revealTx", revealTxHash.String(), "inscription", inscription, "fees", fees)

	return nil
}

// DecodeInscription reads the inscribed message from BTC by a transaction hash
func (client *Client) DecodeInscription(revealTxHash string) (string, error) {
	tx, err := client.GetTransaction(revealTxHash, false)
	if err != nil {
		return "", err
	}
	inscriptionMessage, err := client.getInscriptionMessage(tx.Hex)
	if err != nil {
		return "", err
	}

	disasm, err := txscript.DisasmString(inscriptionMessage)
	if err != nil {
		return "", err
	}

	proof := strings.ReplaceAll(disasm, " ", "")

	return proof, nil
}

// getTransaction returns a transaction from BTC by a transaction hash
func (client *Client) GetTransaction(txid string, verbose bool) (*btcjson.TxRawResult, error) {
	return client.IndexerClient.GetTransaction(context.Background(), txid, verbose)
}

// getInscriptionMessage returns the raw inscribed message from the transaction
func (client *Client) getInscriptionMessage(txHex string) ([]byte, error) {
	txBytes, err := hex.DecodeString(txHex)
	if err != nil {
		return nil, err
	}
	var targetTx wire.MsgTx

	err = targetTx.Deserialize(bytes.NewReader(txBytes))
	if err != nil {
		return nil, err
	}
	if len(targetTx.TxIn) < 1 || len(targetTx.TxIn[0].Witness) < 2 {
		return nil, err
	}
	inscriptionHex := hex.EncodeToString(targetTx.TxIn[0].Witness[1])

	const (
		utfMarker       = "6170706c69636174696f6e2f6f637465742d73747265616d" // application/octet-stream
		utfMarkerLength = 48
	)

	// Get the message from the inscription
	markerIndex := strings.Index(inscriptionHex, utfMarker)
	if markerIndex == -1 {
		return nil, fmt.Errorf("inscription hex is invalid")
	}
	messageIndex := markerIndex + utfMarkerLength

	messageHex := inscriptionHex[messageIndex : len(inscriptionHex)-2]
	decodedBytes, err := hex.DecodeString(messageHex)
	if err != nil {
		return nil, err
	}
	return decodedBytes, nil
}

// GetBlockchainHeight returns the current height of the btc blockchain
func (client *Client) GetBlockchainHeight() (int32, error) {
	blockChainInfo, err := client.IndexerClient.GetBlockchainInfo(context.Background())
	if err != nil {
		return -1, err
	}
	return blockChainInfo.Height, nil
}

// listUnspent returns a list of unsent utxos filtered by address
func (client *Client) ListUnspent() ([]*indexer.UTXO, error) {
	indexerResponse, err := client.IndexerClient.ListUnspent(context.Background(), client.keychain.GetPublicKey())
	if err != nil {
		return nil, err
	}
	blockchainHeight, err := client.GetBlockchainHeight()
	if err != nil {
		return nil, err
	}

	// TODO change and move to config when no longer using coinbase transactions for testing
	requiredCoinbaseConfirmations := int32(100)
	utxos := []*indexer.UTXO{}
	for _, r := range indexerResponse {
		// blockchain height - transacton block height + 1 in order to count the block of the transaction
		confirmations := blockchainHeight - int32(r.Height) + 1
		if confirmations > requiredCoinbaseConfirmations {
			utxos = append(utxos, r)

		}
	}

	return utxos, nil
}

// GetHistory return the confirmed history of the scripthash
func (client *Client) GetHistory() ([]*indexer.Transaction, error) {
	indexerResponse, err := client.IndexerClient.GetHistory(context.Background(), client.keychain.GetPublicKey())
	if err != nil {
		return nil, err
	}
	blockchainHeight, err := client.GetBlockchainHeight()
	if err != nil {
		return nil, err
	}

	utxos := []*indexer.Transaction{}
	for _, r := range indexerResponse {
		// blockchain height - transacton block height + 1 in order to count the block of the transaction
		confirmations := blockchainHeight - int32(r.Height) + 1
		if confirmations > 0 {
			utxos = append(utxos, r)
		}
	}

	return utxos, nil
}

// createRawTransaction returns an unsigned transaction
func (client *Client) createRawTransaction(inputs []btcjson.TransactionInput, outputAmount *btcutil.Amount, outputAddress *btcutil.Address) (*wire.MsgTx, error) {
	tx := wire.NewMsgTx(wire.TxVersion)

	for _, input := range inputs {
		hash, err := chainhash.NewHashFromStr(input.Txid)
		if err != nil {
			return nil, fmt.Errorf("error parsing txid: %v", err)
		}

		outputIndex := input.Vout
		txIn := wire.NewTxIn(wire.NewOutPoint(hash, uint32(outputIndex)), nil, nil)
		tx.AddTxIn(txIn)
	}
	pubKeyHash := (*outputAddress).ScriptAddress()
	witnessProgram := append([]byte{0x00, 0x14}, pubKeyHash...)

	txOut := wire.NewTxOut(int64(*outputAmount), witnessProgram)
	tx.AddTxOut(txOut)

	return tx, nil
}
