package btcman

// BtcmanMode is the mode of the btcman
type BtcmanMode string

const (
	ReaderMode  BtcmanMode = "reader"
	WriterMode  BtcmanMode = "writer"
	InvalidMode BtcmanMode = "invalid"
)
