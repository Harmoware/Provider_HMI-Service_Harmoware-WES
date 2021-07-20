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

func (w *WorkerInfo) OKBatch() error {
	if w.CurrentBatch == nil {
		return nil
	}

	if w.CurrentBatch.itemIndex < len(w.CurrentBatch.Items) {
		return errors.New("worker: please pick all item in your batch")
	}
	return nil
}

func (w *WorkerInfo) SetBatch(b *BatchInfo) {
	w.WorkBatchID = b.ID
	w.CurrentBatch = b
	b.WorkerID = w.ID

	w.CurrentItem = w.CurrentBatch.Items[w.CurrentBatch.itemIndex]
	w.CurrentBatch.StartTime = time.Now()
}

func (w *WorkerInfo) Connect() {
	w.connection = true
	w.lastConnectionTime = time.Now()
}

func (w *WorkerInfo) DisConnect() {
	w.connection = false
}

func (w *WorkerInfo) NextItem() (*ItemInfo, error) {
	if w.CurrentItem == nil {
		return nil, errors.New("worker: not working any batch")
	}
	ne, last := w.CurrentBatch.Next()
	if last {
		ship := new(ItemInfo)
		ship.BatchID = ne.BatchID
		ship.ID = -1
		ship.Name = "shipA"
		ship.Pos = w.CurrentBatch.ShipmentPos
		ship.Shelf = "---"
		return ship, nil
	} else {
		w.CurrentItem = ne
	}
	return ne, nil
}

func (w *WorkerInfo) FinishBatch() error {
	if w.CurrentBatch == nil {
		return errors.New("worker: not working any batch")
	}
	if w.CurrentBatch.itemIndex < len(w.CurrentBatch.Items)-1 {
		return errors.New("worker: please pick all item in your batch")
	}
	w.CurrentBatch.Finish()
	w.CurrentBatch = nil
	w.CurrentItem = nil
	return nil
}
