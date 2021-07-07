package picking

import (
	"errors"
	"time"
)

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
	b.WorkerID = w.ID
}

func (w *WorkerInfo) Connect() {
	w.connection = true
	w.lastConnectionTime = time.Now()
}

func (w *WorkerInfo) DisConnect() {
	w.connection = false
}

func (w *WorkerInfo) NextItem() error {
	if w.CurrentItem == nil {
		return errors.New("not working now")
	}
	ne := w.CurrentBatch.Next()
	if ne == nil {
		return errors.New("no next item")
	} else {
		w.CurrentItem = ne
	}
	return nil
}
