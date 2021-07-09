package picking

import (
	"errors"

	astar "github.com/fukurin00/astar_golang"
	_ "github.com/jbuchbinder/gopnm"
)

type WayPoint [][2]float64

var Routing *astar.Astar

func SetupRouting(fname string) error {
	objs, err := astar.ObjectsFromImage(fname, 100, -41, -81.959, 0.08)
	if err != nil {
		return err
	}
	Routing = astar.NewAstar(objs, 0.3, 0.1)
	return nil
}

//furture work
func GetPath(sx, sy float64, next *ItemInfo) (WayPoint, error) {
	if Routing == nil {
		return nil, errors.New("route: cannot path plann")
	}
	w, err := Routing.Plan(sx, sy, next.Pos.X, next.Pos.Y)
	if err != nil {
		return nil, errors.New("route: fail path planning")
	}
	return w, nil
}
