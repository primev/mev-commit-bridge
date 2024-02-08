package listener

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

type Chain int

const (
	Settlement Chain = iota
	L1
)

func (c Chain) String() string {
	switch c {
	case Settlement:
		return "Settlement"
	case L1:
		return "L1"
	default:
		return "unknown"
	}
}

type TransferInitiatedEvent struct {
	Sender      common.Address
	Recipient   common.Address
	Amount      *big.Int
	TransferIdx *big.Int
	Chain       Chain
}

type TransferFinalizedEvent struct {
	Recipient       common.Address
	Amount          *big.Int
	CounterpartyIdx *big.Int
	Chain           Chain
}
