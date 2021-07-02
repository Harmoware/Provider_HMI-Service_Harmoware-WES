package picking

import (
	"time"

	wes "github.com/synerex/proto_wes"
)

type Pos struct {
	X float64
	Y float64
}

type BatchInfo struct {
	ID           int64
	WorkerID     int64
	Floor        int
	ShipmentPos  Pos
	Items        []*ItemInfo
	StartTime    time.Time
	ShipmentTime time.Time

	itemIndex int
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
	if current.Batch.itemIndex >= len(current.Batch.Items) {
		return nil
	}
	current.Batch.itemIndex++
	return current.Batch.Items[current.Batch.itemIndex]
}

// furture work
func (b *BatchInfo) SortItems() {

}
