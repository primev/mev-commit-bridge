package listener

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
	Sender      string
	Recipient   string
	Amount      uint64
	TransferIdx uint64
	Chain       Chain
}

type TransferFinalizedEvent struct {
	Recipient       string
	Amount          uint64
	CounterpartyIdx uint64
	Chain           Chain
}
