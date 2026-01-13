package models

import "time"

type EventLog struct {
	ID          uint
	TxHash      string
	LogIndex    uint
	BlockNumber uint64
	Event       string
	Contract    string
	EventArgs   string
	CreatedAt   time.Time
}

func (EventLog) TableName() string {
	return "event_log"
}
