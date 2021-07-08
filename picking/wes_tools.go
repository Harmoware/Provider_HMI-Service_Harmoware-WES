package picking

import (
	"encoding/csv"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/synerex/proto_wes"
)

func getShelfMap(fname string) map[string]Pos {
	var shelfMap map[string]Pos
	shelfMap = make(map[string]Pos)

	f, err := os.Open(fname)
	if err != nil {
		log.Fatal("cannot read file: ", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)

	content, err := reader.ReadAll()
	if err != nil {
		log.Print(err)
	}

	for i, row := range content {
		for _, s := range row { //空白文字を取り除く
			s = strings.Replace(s, " ", "", -1)
		}
		if i == 0 {
			continue //skip header
		}

		x, err1 := strconv.ParseFloat(row[1], 64)
		y, err2 := strconv.ParseFloat(row[2], 64)
		if err1 != nil || err2 != nil {
			log.Printf("cannot parse locmap index%d: ", i, err1, err2)
		}
		shelfMap[row[0]] = Pos{X: float64(x), Y: float64(y)}
	}
	return shelfMap
}

func loadCsv(fname string) [][]string {
	f, err := os.Open(fname)
	if err != nil {
		log.Fatal("file loading failer: ", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)

	content, err := reader.ReadAll()
	if err != nil {
		log.Print(err)
	}
	//スペースを取り除く
	for _, row := range content {
		for _, s := range row {
			s = strings.Replace(s, " ", "", -1)
		}
	}
	return content
}

// orderを送信する、1バッチずつ送信する
func ReadWmsCsv(wmsFile string) []*proto_wes.WmsOrder {
	itemsMap := make(map[int][]*proto_wes.Item)

	content := loadCsv(wmsFile)
	var idList []int
	idHumanMap := make(map[int]int)

	wmsIDIndex := 0
	shelfIDIndex := 2
	humanIDIndex := -1
	//isFullIndex := 4

	for i, row := range content {
		if i == 0 {
			for j, head := range row {
				switch strings.ToLower(head) {
				case "batid", "bat_id", "id", "wmsid", "wms_id":
					wmsIDIndex = j
				case "shelfid", "shelf_id", "location":
					shelfIDIndex = j
				case "isfull":
					//isFullIndex = j
				case "userid", "human":
					humanIDIndex = j
				}
			}
			continue
		}

		wmsID, err := strconv.Atoi(row[wmsIDIndex])

		if err != nil {
			log.Print(err)
		}

		if humanIDIndex != -1 {
			humanID, herr := strconv.Atoi(row[humanIDIndex])
			// log.Printf("wms%d send to specific human%d", wmsID, humanID)
			if herr != nil {
				log.Print(herr)
			} else {
				idHumanMap[wmsID] = humanID
			}
		} else {
			idHumanMap[wmsID] = -1
		}

		shelfID := row[shelfIDIndex]

		item := proto_wes.Item{
			ShelfID: shelfID,
		}

		if _, ok := itemsMap[wmsID]; !ok {
			idList = append(idList, wmsID)
		}
		itemsMap[wmsID] = append(itemsMap[wmsID], &item)
	}

	out := make([]*proto_wes.WmsOrder, len(idList))
	for i, id := range idList {
		rcd := &proto_wes.WmsOrder{
			WmsID:   int64(id),
			HumanID: int64(idHumanMap[id]),
			Item:    itemsMap[id],
		}
		out[i] = rcd
	}
	return out
}
