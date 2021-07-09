package picking

import (
	"errors"
	"flag"
	"fmt"
	"time"

	wes "github.com/synerex/proto_wes"
)

var (
	Locmap       map[string]Pos
	LocationFile = flag.String("locmap", "assets/location_list.csv", "location list csv file")
	MapFile      = flag.String("map", "assets/map.pgm", "map file for routing")
)

type Pos struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type BatchStatus struct {
	BatchList []*BatchInfo
	batchnum  int
	frees     []int
	progress  []int
	idLast    int
}

func NewBatchStatus() *BatchStatus {
	bs := new(BatchStatus)
	bs.BatchList = make([]*BatchInfo, 0)
	bs.frees = make([]int, 0)
	bs.progress = make([]int, 0)
	bs.batchnum = 0
	Locmap = getShelfMap(*LocationFile)
	bs.idLast = 0

	// log.Print("loading mapfile ", *MapFile, " ...")
	// err := SetupRouting(*MapFile)
	// log.Print("load mapfile")
	// if err != nil {
	// 	log.Print("batchStatus: ", err)
	// }
	return bs
}

func (bs *BatchStatus) AddBatch(b *BatchInfo) {
	bs.BatchList = append(bs.BatchList, b)
	bs.batchnum++
	bs.frees = append(bs.frees, bs.batchnum-1)
}

func (bs *BatchStatus) AssignBatch() (*BatchInfo, error) {
	if len(bs.frees) == 0 {
		return nil, errors.New("start: no batch")
	}
	// todo 何を割り振るか決める
	id := bs.frees[0]
	if len(bs.frees) >= 1 {
		bs.frees = bs.frees[1:]
	}
	bs.progress = append(bs.progress, id)
	return bs.BatchList[id], nil
}

func (bs *BatchStatus) ReadOrder(fname string) {
	orders := ReadWmsCsv(fname)

	for _, rcd := range orders {
		newB := bs.NewBatchInfo(rcd)
		bs.AddBatch(newB)
	}
}

func (bs *BatchStatus) NewBatchInfo(rcd *wes.WmsOrder) *BatchInfo {
	b := new(BatchInfo)
	bs.idLast++
	b.ID = int64(bs.idLast) + 100*rcd.WmsID
	b.WorkerID = -1
	b.Floor = 3
	b.ShipmentPos = Pos{X: 7.0, Y: 8.0}
	b.Items = make([]*ItemInfo, 0)

	b.itemIndex = 0

	for i, item := range rcd.Item {
		name := fmt.Sprintf("test_order%d_%d", b.ID, i)
		b.Items = append(b.Items, NewItemInfo(item, int64(i), name, b))
	}
	b.SortItems()
	return b
}

type BatchInfo struct {
	ID          int64       `json:"id"`
	WorkerID    int64       `json:"worker_id"`
	Floor       int         `json:"floor"`
	ShipmentPos Pos         `json:"ship_pos"`
	Items       []*ItemInfo `json:"items"`
	StartTime   time.Time   `json:"start_time"`

	shipmentTime time.Time
	itemIndex    int
}

// next item
func (current *BatchInfo) Next() (ni *ItemInfo, ship bool) {
	if current.itemIndex >= len(current.Items)-1 {
		return current.Items[len(current.Items)-1], true
	}
	current.Items[current.itemIndex].picked = true
	current.Items[current.itemIndex].PickTime = time.Now()
	current.itemIndex++
	return current.Items[current.itemIndex], false
}

// todo
func (b *BatchInfo) SortItems() {

}

func (b *BatchInfo) Finish() {
	b.shipmentTime = time.Now()
}

type ItemInfo struct {
	Name     string    `json:"name"`
	Pos      Pos       `json:"position"`
	Shelf    string    `json:"shelf"`
	ID       int64     `json:"id"`
	BatchID  int64     `json:"batch_id"`
	PickTime time.Time `json:"pick_time"`

	batch  *BatchInfo
	picked bool
}

func NewItemInfo(rev *wes.Item, id int64, name string, b *BatchInfo) *ItemInfo {
	i := new(ItemInfo)
	i.ID = id
	i.picked = false
	i.BatchID = b.ID
	i.Name = name
	i.batch = b
	i.Pos = Locmap[rev.ShelfID]
	i.Shelf = rev.ShelfID
	return i
}
