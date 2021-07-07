package picking

import (
	"time"

	wes "github.com/synerex/proto_wes"
)

var (
	Locmap map[string]Pos
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
}

func NewBatchStatus() *BatchStatus {
	bs := new(BatchStatus)
	bs.batchList = make([]*BatchInfo, 0)
	bs.batchnum = 0
	Locmap = getShelfMap("../assets/location_list.csv")
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
	b.Floor = 3
	b.ShipmentPos = Pos{X: 7.0, Y: 8.0}
	b.Items = make([]*ItemInfo, 0)

	b.itemIndex = 0

	for i, item := range rcd.Item {
		b.Items = append(b.Items, NewItemInfo(item, int64(i), b))
	}
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

func NewItemInfo(rev *wes.Item, id int64, b *BatchInfo) *ItemInfo {
	i := new(ItemInfo)
	i.ID = id
	i.picked = false
	i.BatchID = b.ID
	i.Batch = b
	i.Pos = Locmap[rev.ShelfID]
	i.Shelf = rev.ShelfID
	return i
}

// next item
func (current *BatchInfo) Next() *ItemInfo {
	if current.itemIndex >= len(current.Items)-1 {
		return nil
	}
	current.Items[current.itemIndex].picked = true
	current.Items[current.itemIndex].PickTime = time.Now()
	current.itemIndex++
	return current.Items[current.itemIndex]
}

// todo
func (b *BatchInfo) SortItems() {

}
