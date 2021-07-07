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

func NewWorkerInfo(id int64) *WorkerInfo {
	w := new(WorkerInfo)
	w.ID = id
	return w
}

func (w *WorkerInfo) SetBatch(b *BatchInfo) {
	w.WorkBatchID = b.ID
	w.CurrentBatch = b
}

func (w *WorkerInfo) Connect() {
	w.connection = true
}

func (w *WorkerInfo) DisConnect() {
	w.connection = false
}
