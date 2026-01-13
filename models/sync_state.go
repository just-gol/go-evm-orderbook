package models

type SyncState struct {
	ID          uint
	Name        string
	BlockNumber uint64
}

func (SyncState) TableName() string {
	return "sync_state"
}
