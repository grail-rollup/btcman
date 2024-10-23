package btcman

type Config struct {
	// Mode is the mode of the btcman: reader or writer
	Mode string `mapstructure:"Mode"`

	// Net is the type of network of the btc node
	Net string `mapstructure:"Net"`

	// PrivateKey is the private key for the btc node wallet, required only for writer mode
	PrivateKey string `mapstructure:"PrivateKey"`

	// PublicKey is the public key for the btc node wallet, required only for reader mode
	PublicKey string `mapstructure:"PublicKey"`

	// IndexerHost is the host of the indexer server
	IndexerHost string `mapstructure:"IndexerHost"`

	// IndexerPort is the port of the indexer server
	IndexerPort string `mapstructure:"IndexerPort"`

	// ConsolidationInterval is the interval between checks for utxos consolidations, in seconds
	ConsolidationInterval int `mapstructure:"ConsolidationInterval"`

	// ConsolidationTransactionFee is the fee paid for the consolidation transaction, in satoshi
	ConsolidationTransactionFee int `mapstructure:"ConsolidationTransactionFee"`

	// UtxoThreshold is the the minimum amount of satoshis under which the UTXO is used for consolidation
	UtxoThreshold int `mapstructure:"UtxoThreshold"`

	// MinUtxoConsolidationAmount is the minimum number of UTXOS under the UtxoThreshold in order to perform a consolidation
	MinUtxoConsolidationAmount int `mapstructure:"MinUtxoConsolidationAmount"`

	// EnableIndexerDebug is a flag for enabling debuging messages in indexer client
	EnableIndexerDebug bool `mapstructure:"EnableIndexerDebug"`
}

func IsValidBtcConfig(cfg *Config) bool {
	return cfg.Mode != "" &&
		cfg.Net != "" &&
		(cfg.PrivateKey != "" || cfg.PublicKey != "") &&
		cfg.IndexerHost != "" &&
		cfg.IndexerPort != ""
}
