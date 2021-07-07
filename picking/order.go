package picking

import (
	"time"

	wes "github.com/synerex/proto_wes"
)

type Pos struct {
	X float64
	Y float64
}

type BatchStatus struct {
	batchList []*BatchInfo

	batchnum int
	frees    []int
	progress []int
	finishs  []int
}

func NewBatchStatus() *BatchStatus {
	bs := new(BatchStatus)
	bs.batchList = make([]*BatchInfo, 0)
	bs.batchnum = 0
	return bs
}

func (bs *BatchStatus) AddBatch(b *BatchInfo) {
	bs.batchList = append(bs.batchList, b)
	bs.batchnum++
	bs.frees = append(bs.frees, bs.batchnum-1)
}

func (bs *BatchStatus) AssignBatch() *BatchInfo {
	if len(bs.frees) == 0 {
		return nil
	}
	// todo 何を割り振るか決める
	id := bs.frees[0]
	bs.frees = bs.frees[1:]
	bs.progress = append(bs.progress, id)
	return bs.batchList[id]
}

type BatchInfo struct {
	ID           int64       `json: "id"`
	WorkerID     int64       `json: "worker_id"`
	Floor        int         `json: "floor"`
	ShipmentPos  Pos         `json: "ship_pos"`
	Items        []*ItemInfo `json: "items"`
	StartTime    time.Time   `json: "start_time"`
	ShipmentTime time.Time

	itemIndex int
}

func NewBatchInfo(rcd *wes.WmsOrder) *BatchInfo {
	b := new(BatchInfo)
	b.ID = rcd.WmsID
	b.WorkerID = -1
	b.Floor = 2
	b.ShipmentPos = Pos{X: 7.0, Y: 8.0}
	b.Items = make([]*ItemInfo, 0)

	b.itemIndex = 0
	return b
}

type ItemInfo struct {
	Name      string
	Pos       Pos
	Shelf     string
	ID        int64
	BatchID   int64
	Batch     *BatchInfo
	StartTime time.Time
	PickTime  time.Time

	picked bool
}

func NewItemInfo(rev wes.Item) *ItemInfo {
	i := new(ItemInfo)
	i.picked = false

	return i
}

// next item
func (current *ItemInfo) Next() *ItemInfo {
	if current.Batch.itemIndex >= len(current.Batch.Items)-1 {
		return nil
	}
	current.Batch.itemIndex++
	return current.Batch.Items[current.Batch.itemIndex]
}

// todo
func (b *BatchInfo) SortItems() {

}
