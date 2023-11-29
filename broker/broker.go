package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"sync"
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

type GolOp struct {
	completedTurn int
	cellCount     int
	world         [][]uint8
	pause         chan bool
	needResume    bool
	needSave      chan bool
	//worldBeforeQuit [][]uint8
	//turnBeforeQuit  int

}

var mutex sync.Mutex

func (g *GolOp) ExecuteTurns(req gol.Request, res *gol.Response) (err error) {
	mutex.Lock()
	g.needSave = make(chan bool, 1)
	g.pause = make(chan bool, 1)
	//turn := 0 need to un comment when change to extention mode(game can be saved and continue)
	g.world = initWorld(req.P.ImageHeight, req.P.ImageWidth)
	res.NewWorld = initWorld(req.P.ImageHeight, req.P.ImageWidth)
	copyWhole(g.world, req.World)
	g.completedTurn = 0
	mutex.Unlock()
	if req.P.Turns == 0 {
		copyWhole(res.NewWorld, req.World)
		//res.NewWorld = req.World
		res.Final = gol.FinalTurnComplete{CompletedTurns: req.P.Turns, Alive: calculateAliveCells(g.world)}
		return
	} else {
		//workers := []string{"54.227.67.232:8030", "3.90.111.104:8030", "54.237.222.162:8030", "3.87.240.120:8030"}
		workers := []string{"35.172.226.22:8030", "54.84.139.55:8030", "54.82.127.203:8030", "52.91.247.6:8030"}
		//workers := []string{"127.0.0.1:8031", "127.0.0.1:8032", "127.0.0.1:8033", "127.0.0.1:8034"}
		clients := make([]*rpc.Client, len(workers))
		for i, worker := range workers {
			client, _ := rpc.Dial("tcp", worker)
			defer client.Close()
			clients[i] = client
		}
		for t := 0; t < req.P.Turns; t++ { //t need to be turn when extention mode
			select {
			case <-g.pause:
				pause := <-g.pause
				if !pause {
					break
				}
			case <-g.needSave:
				fmt.Println("q is pressed, the process pop off")
				return
			default:
				break
			}
			allPartsChan := make(chan [][]uint8, 1)
			partHeight := req.P.ImageHeight / len(clients)
			//worker1:=worker{
			//	worldPart: make([][]uint8, partHeight+2),
			//}
			for i := 0; i < len(clients); i++ {

				go func(i int) {
					partInfo := gol.PartInfo{g.world, i * partHeight, (i + 1) * partHeight, req.P.ImageWidth}
					newPart := new(gol.NewPart)
					clients[i].Call(gol.WorkerProcess, partInfo, newPart)
					mutex.Lock()
					for h := i * partHeight; h < (i+1)*partHeight; h++ {
						copy(res.NewWorld[h], newPart.NewWorldPart[h])
					}
					allPartsChan <- res.NewWorld
					mutex.Unlock()
				}(i)
			}
			for i := 0; i < len(clients); i++ {
				<-allPartsChan
			}
			//partInfo := gol.PartInfo{g.world, 0, req.P.ImageHeight, req.P.ImageWidth}
			//newPart := new(gol.NewPart)
			//client1.Call(gol.WorkerProcess, partInfo, newPart)
			//res.NewWorld = newPart.NewWorldPart
			mutex.Lock()
			copyWhole(g.world, res.NewWorld)
			g.completedTurn = t + 1
			//fmt.Println("in loop, turn completed:", g.completedTurn)
			mutex.Unlock()
		}

		res.Final = gol.FinalTurnComplete{CompletedTurns: req.P.Turns, Alive: calculateAliveCells(res.NewWorld)}
		fmt.Println("game/test finished")
		return
	}

}
func (g *GolOp) Timer(req gol.Request, res *gol.ReportAlive) (err error) {
	mutex.Lock()
	fmt.Println("reported in turn", g.completedTurn)
	res.Alive = gol.AliveCellsCount{CellsCount: len(calculateAliveCells(g.world)), CompletedTurns: g.completedTurn}
	mutex.Unlock()
	return
}

func (g *GolOp) KeyOp(op gol.KeyPress, res *gol.Response) (err error) {
	res.NewWorld = initWorld(op.P.ImageHeight, op.P.ImageWidth)
	switch op.Key {
	case 's':
		// Save the game state
		fmt.Println("s is pressed, the instant is saved")
		mutex.Lock()
		copyWhole(res.NewWorld, g.world)
		res.CurrentTurn = g.completedTurn
		fmt.Println("saved the current state at turn:", g.completedTurn)
		mutex.Unlock()
	case 'k':
		fmt.Println("k is pressed, the game is saved")
		mutex.Lock()
		copyWhole(res.NewWorld, g.world)
		res.CurrentTurn = g.completedTurn
		res.Final = gol.FinalTurnComplete{CompletedTurns: g.completedTurn, Alive: calculateAliveCells(res.NewWorld)}
		fmt.Println("saved the current state at turn:", g.completedTurn)
		mutex.Unlock()
	case 'p':
		fmt.Println("p is preesed, the instant is saved")
		mutex.Lock()
		copyWhole(res.NewWorld, g.world)
		res.CurrentTurn = g.completedTurn
		fmt.Println("saved the current state at turn:", g.completedTurn)
		g.pause <- true
		mutex.Unlock()
	case 'q':
		fmt.Println("q is pressed, the instant should be saved and continue next time")
		mutex.Lock()
		g.needSave <- true
		g.needResume = true
		mutex.Unlock()
	}
	return
}

func (g *GolOp) Kill(op gol.KeyPress, res *gol.Response) (err error) {
	os.Exit(0)
	return
}

func (g *GolOp) Resume(op gol.KeyPress, res *gol.Response) (err error) {
	fmt.Println("Resume")
	g.pause <- false
	return
}
func (g *GolOp) Live(req gol.Request, res *gol.Response) (err error) {
	res.NewWorld = initWorld(req.P.ImageHeight, req.P.ImageWidth)
	mutex.Lock()
	copyWhole(res.NewWorld, g.world)
	res.CurrentTurn = g.completedTurn
	mutex.Unlock()
	return
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
	fmt.Println("connected")
}

func initWorld(height, width int) [][]uint8 {
	world := make([][]uint8, height)
	for i := range world {
		world[i] = make([]uint8, width)
	}
	return world
}

type worker struct {
	worldPart [][]uint8
	upperHalo []uint8
	lowerHalo []uint8
}
