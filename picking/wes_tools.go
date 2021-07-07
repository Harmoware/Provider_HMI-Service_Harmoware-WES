package picking

import (
	"encoding/csv"
	"log"
	"os"
	"strconv"
	"strings"
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
