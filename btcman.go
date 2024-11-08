package btcman

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/grail-rollup/btcman/common"
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
	logger                   log.Logger
	keychain                 Keychainer
	netParams                *chaincfg.Params
	cfg                      Config
	address                  *btcutil.Address
	IndexerClient            indexer.Indexerer
	consolidationStopChannel chan struct{}
	utxoThreshold            float64
	isDebug                  bool
}

func NewClient(cfg Config) (Clienter, error) {
	logger := log.New("module", common.BTCMAN)
	logger.SetHandler(log.StreamHandler(os.Stdout, log.TerminalFormat()))
	isDebug := cfg.EnableDebug

	if isDebug {
		logger.Debug("Creating btcman")
	}

	isValid := IsValidBtcConfig(&cfg)
	if !isValid {
		return nil, errors.New("invalid config")
	}

	// Load default consolidation values
	consolidationInterval, consolidationTransactionFee, utxoThreshold, minUtxoConsolidationAmount := loadConsolidationValues(&cfg)
	// Load network
	network, err := loadNetwork(cfg.Net)
	if err != nil {
		return nil, err
	}
	// Load mode
	mode, err := loadMode(cfg.Mode)
	if err != nil {
		return nil, err
	}

	keychain, err := NewKeychain(&cfg, mode, network, logger)
	if err != nil {
		return nil, err
	}
	address, err := indexer.PublicKeyToAddress(keychain.GetPublicKey(), network)
	if err != nil {
		return nil, err
	}

	indexer := indexer.NewIndexer(isDebug, logger)
	indexer.Start(fmt.Sprintf("%s:%s", cfg.IndexerHost, cfg.IndexerPort))

	stopChannel := make(chan struct{})

	btcman := Client{
		logger:                   logger,
		keychain:                 keychain,
		cfg:                      cfg,
		netParams:                network,
		address:                  &address,
		IndexerClient:            indexer,
		consolidationStopChannel: stopChannel,
		utxoThreshold:            float64(utxoThreshold),
		isDebug:                  isDebug,
	}

	if mode == WriterMode {
		ticker := time.NewTicker(time.Second * time.Duration(consolidationInterval))

		go func() {
			for {
				select {
				case <-btcman.consolidationStopChannel:
					ticker.Stop()
					return
				case <-ticker.C:
					if isDebug {
						logger.Debug("Trying to consolidate")
					}
					utxos, err := btcman.ListUnspent()
					if err != nil {
						logger.Error("Failed to list utxos", "err", err)
					}

					btcman.consolidateUTXOS(utxos, float64(consolidationTransactionFee), minUtxoConsolidationAmount)
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

	utxoIndex := client.getIndexOfUtxoAboveThreshold(client.utxoThreshold, utxos)
	if utxoIndex == -1 {
		return nil, fmt.Errorf("can't find utxo to inscribe")
	}

	utxo := utxos[utxoIndex]

	client.logger.Info("UTXO for address was found")
	return utxo, nil
}

// consolidateUTXOS combines multiple utxo in one if the utxos are under a specific threshold and over a specific count
func (client *Client) consolidateUTXOS(utxos []*indexer.UTXO, consolidationFee float64, minUtxoCountConsolidate int) {
	if len(utxos) == 0 {
		client.logger.Info("Address has zero utxos.. skipping consolidation")
		return
	}

	var inputs []btcjson.TransactionInput
	dustAmount := btcutil.Amount(546)
	totalAmount := btcutil.Amount(0)

	for _, utxo := range utxos {
		amount := btcutil.Amount(utxo.Value)
		thresholdAmount := btcutil.Amount(client.utxoThreshold)
		if amount < thresholdAmount && amount > dustAmount {
			inputs = append(inputs, btcjson.TransactionInput{
				Txid: utxo.TxHash,
				Vout: uint32(utxo.TxPos),
			})
			if client.isDebug {
				client.logger.Debug("Adding utxo", "hash", utxo.TxHash, "amount", amount)
			}
			totalAmount += amount
		}
	}

	if len(inputs) < minUtxoCountConsolidate || totalAmount <= btcutil.Amount(consolidationFee) {
		client.logger.Info("Not enough UTXOs under the specified amount to consolidate.", "utxos",
			len(inputs), "minUtxoCount", minUtxoCountConsolidate, "utxoThreshold", client.utxoThreshold)
		return
	}

	client.logger.Info("Consolidating utxos", "utxos", len(inputs), "amount", totalAmount)

	outputAmount := totalAmount - btcutil.Amount(consolidationFee*(float64(len(inputs))*0.1))

	rawTx, err := client.createRawTransaction(inputs, &outputAmount, client.address)
	if err != nil {
		client.logger.Error("error creating raw transaction", "err", err)
		return
	}

	err = client.keychain.SignTransaction(rawTx, client.IndexerClient)
	if err != nil {
		client.logger.Error("error signing raw transaction", "err", err)
		return
	}

	txHash, err := client.IndexerClient.SendTransaction(context.Background(), rawTx)
	if err != nil {
		client.logger.Error("error sending transaction", "err", err)
		return
	}
	client.logger.Info("UTXOs consolidated successfully", "txHash", txHash)
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
		client.logger.Error("Failed to create inscription request")
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

	if client.isDebug {
		client.logger.Debug("Successful inscription", "commitTx", commitTxHash.String(),
			"revealTx", revealTxHash.String(), "inscription", inscription, "fees", fees)
	}

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

// GetHistory returns the confirmed history of the scripthash, starting from the startHeight if > 1
func (client *Client) GetHistory(startHeight int, includeMempool bool) ([]*indexer.Transaction, error) {
	transactions, err := client.IndexerClient.GetHistory(context.Background(), client.keychain.GetPublicKey())
	if err != nil {
		return nil, err
	}
	ReverseTransactionList(transactions)

	mempoolIndex := -1
	for index := range transactions {
		if transactions[index].Height > 0 {
			mempoolIndex = index
			break
		}
	}
	var mempoolTransactions []*indexer.Transaction
	var confirmedTransactions []*indexer.Transaction
	if mempoolIndex != -1 {
		confirmedTransactions = transactions[mempoolIndex:]
		mempoolTransactions = transactions[:mempoolIndex]
	} else {
		confirmedTransactions = transactions
		mempoolTransactions = []*indexer.Transaction{}
	}
	ReverseTransactionList(confirmedTransactions)

	if startHeight > 1 {
		blockchainHeight, err := client.GetBlockchainHeight()
		if err != nil {
			return nil, err
		}
		if startHeight > int(blockchainHeight) {
			return nil, fmt.Errorf("start height is greater than the blockchain height")
		}

		startHeightIndex := getStartHeightIndex(confirmedTransactions, startHeight)
		if startHeightIndex == -1 {
			client.logger.Warn("no transactions found beyond specified start height", "startHeight", startHeight)
			return []*indexer.Transaction{}, nil
		}

		confirmedTransactions = confirmedTransactions[startHeightIndex:]
	}

	if includeMempool {
		transactions = append(mempoolTransactions, confirmedTransactions...)
	} else {
		transactions = confirmedTransactions
	}

	return transactions, nil
}

func ReverseTransactionList(slice []*indexer.Transaction) {
	for i, j := 0, len(slice)-1; i < j; i, j = i+1, j-1 {
		slice[i], slice[j] = slice[j], slice[i]
	}
}

// getStartHeightIndex returns the index of the transaction with the target height
func getStartHeightIndex(transactions []*indexer.Transaction, targetHeight int) int {
	targetIndex := -1
	left, right := 0, len(transactions)-1
	for left <= right {
		mid := (left + right) / 2
		if transactions[mid].Height == int32(targetHeight) {
			targetIndex = mid
			break
		}
		if transactions[mid].Height < int32(targetHeight) {
			left = mid + 1
		} else {
			right = mid - 1
		}
	}
	return targetIndex
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

// GetBlockHeader returns a block header struct by a given block height
func (client *Client) GetBlockHeader(height uint64) (*wire.BlockHeader, error) {
	blockHeaderHex, err := client.IndexerClient.GetBlockHeader(context.Background(), height)
	if err != nil {
		return nil, err
	}

	data, err := hex.DecodeString(blockHeaderHex)
	if err != nil {
		return nil, err
	}

	var blockHeader wire.BlockHeader
	reader := bytes.NewReader(data)
	err = blockHeader.Deserialize(reader)
	if err != nil {
		return nil, err
	}

	return &blockHeader, nil
}
