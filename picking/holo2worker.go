package picking

import "time"

type WorkerInfo struct {
	ID           int64
	WorkBatchID  int64
	CurrentItem  *ItemInfo
	CurrentBatch *BatchInfo

	connection         bool
	lastConnectionTime time.Time
}

func (w *WorkerInfo) SetBatch(b *BatchInfo) {
	w.WorkBatchID = b.ID
	w.CurrentBatch = b
}
