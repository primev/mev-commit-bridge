package listener

type chain int

const (
	settlement chain = iota
	l1
)

func (c chain) String() string {
	switch c {
	case settlement:
		return "settlement"
	case l1:
		return "l1"
	default:
		return "unknown"
	}
}

type TransferInitiatedEvent struct {
	Sender      string
	Recipient   string
	Amount      uint64
	TransferIdx uint64
	Chain       chain
}

type TransferFinalizedEvent struct {
	Recipient       string
	Amount          uint64
	CounterpartyIdx uint64
	Chain           chain
}
