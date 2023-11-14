package main

import (
	"flag"
	"math/rand"
	"net"
	"net/rpc"
	"time"
	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/util"
)

/** Super-Secret `reversing a string' method we can't allow clients to see. **/
func calculateAliveCells(world [][]uint8) []util.Cell {
	var alive []util.Cell
	for i := 0; i < len(world); i++ {
		for k := 0; k < len(world[i]); k++ {
			if world[i][k] == 255 {
				alive = append(alive, util.Cell{X: k, Y: i})
			}
		}
	}
	return alive
}

type Pair struct {
	y int
	x int
}

func calculateNearAlive(world [][]uint8, row, col int) int {
	adjacent := []Pair{
		{-1, -1}, {-1, 0}, {-1, 1},
		{0, -1}, {0, 1},
		{1, -1}, {1, 0}, {1, 1},
	}
	//add to get all adj nodes within 0-15
	for i := 0; i < 8; i++ {
		adjacent[i].y += row
		if adjacent[i].y == len(world) {
			adjacent[i].y = 0
		}
		if adjacent[i].y == -1 {
			adjacent[i].y = len(world) - 1
		}
		adjacent[i].x += col
		if adjacent[i].x == len(world[i]) {
			adjacent[i].x = 0
		}
		if adjacent[i].x == -1 {
			adjacent[i].x = len(world[i]) - 1
		}
	}
	//count alive using for
	count := 0
	for _, node := range adjacent {
		if world[node.y][node.x] == 255 {
			count++
		}
	}
	return count
}
func calculateNextState(startY, endY int, world [][]uint8) [][]uint8 {

	newWorld := make([][]uint8, len(world))
	for i := range newWorld {
		newWorld[i] = make([]uint8, len(world[i]))
	}
	for i := startY; i < endY; i++ { //each row
		for k := 0; k < len(world[i]); k++ { //each item in row
			numOfAlive := calculateNearAlive(world, i, k)
			currentNode := world[i][k]

			//rules for updating the cell state
			if world[i][k] == 255 {
				if numOfAlive < 2 {
					newWorld[i][k] = 0
				} else if numOfAlive == 2 || numOfAlive == 3 {
					newWorld[i][k] = currentNode
				} else if numOfAlive > 3 {
					newWorld[i][k] = 0
				}
			} else if currentNode == 0 && numOfAlive == 3 {
				newWorld[i][k] = 255
			}
		}
	}

	return newWorld
}
func copyWhole(dst, src [][]uint8) {
	for i := range src {
		copy(dst[i], src[i])
	}
}

type GolOp struct{}

func (g *GolOp) ExecuteTurns(req gol.Request, res *gol.Response) (err error) {
	if req.P.Turns == 0 {
		res.NewWorld = req.World
		res.Final = gol.FinalTurnComplete{CompletedTurns: req.P.Turns, Alive: calculateAliveCells(req.World)}
		return
	} else {
		for t := 0; t < req.P.Turns; t++ {
			res.NewWorld = calculateNextState(0, req.P.ImageHeight, req.World)
			copyWhole(req.World, res.NewWorld)
		}
		res.Final = gol.FinalTurnComplete{CompletedTurns: req.P.Turns, Alive: calculateAliveCells(res.NewWorld)}
		return
	}

}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	rpc.Register(&GolOp{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {

		}
	}(listener)
	rpc.Accept(listener)
}
