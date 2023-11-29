package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"time"
	"uk.ac.bris.cs/gameoflife/gol"
)

func calculateNextState(startY, endY int, world [][]uint8) [][]uint8 {

	newWorld := make([][]uint8, len(world)-2)
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
					newWorld[i-1][k] = 0
				} else if numOfAlive == 2 || numOfAlive == 3 {
					newWorld[i-1][k] = currentNode
				} else if numOfAlive > 3 {
					newWorld[i-1][k] = 0
				}
			} else if currentNode == 0 && numOfAlive == 3 {
				newWorld[i-1][k] = 255
			}
		}
	}

	return newWorld
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
		if adjacent[i].x == len(world[1]) {
			adjacent[i].x = 0
		}
		if adjacent[i].x == -1 {
			adjacent[i].x = len(world[1]) - 1
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

type Worker struct {
	part  [][]uint8
	hight int
}

func (w *Worker) Worker(req gol.HaloReq, res *gol.HaloRes) (err error) {
	if req.WorldPart != nil {
		w.part = initWorld(req.PartHeight, req.Width)
		w.hight = req.PartHeight
		res.WorldPart = initWorld(req.PartHeight, req.Width)
		temp := make([][]uint8, 0, req.PartHeight+2)
		temp = append(temp, req.UpperHalo)
		for _, row := range req.WorldPart {
			temp = append(temp, row)
		}
		temp = append(temp, req.LowerHalo)
		res.WorldPart = calculateNextState(1, req.PartHeight+1, temp)
		copyWhole(w.part, res.WorldPart)
		return
	} else {
		res.WorldPart = initWorld(req.PartHeight, req.Width)
		res.WorldPart = initWorld(req.PartHeight, req.Width)
		temp := make([][]uint8, 0, req.PartHeight+2)
		temp = append(temp, req.UpperHalo)
		for _, row := range w.part {
			temp = append(temp, row)
		}
		temp = append(temp, req.LowerHalo)
		res.WorldPart = calculateNextState(1, req.PartHeight+1, temp)
		copyWhole(w.part, res.WorldPart)
		return
	}

}

func (w *Worker) Kill(op gol.KeyPress, res *gol.Response) (err error) {
	os.Exit(0)
	return
}

func main() {
	//pAddr := flag.String("port", "8030", "Port to listen on")
	pAddr := flag.String("port", "8031", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	rpc.Register(&Worker{hight: 0})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {

		}
	}(listener)
	rpc.Accept(listener)
	fmt.Println("connected")
}

func copyWhole(dst, src [][]uint8) {
	for i := range src {
		copy(dst[i], src[i])
	}
}

func initWorld(height, width int) [][]uint8 {
	world := make([][]uint8, height)
	for i := range world {
		world[i] = make([]uint8, width)
	}
	return world
}
