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
	g.world = make([][]uint8, req.P.ImageHeight)
	res.NewWorld = make([][]uint8, req.P.ImageHeight)
	for i := range g.world {
		g.world[i] = make([]uint8, req.P.ImageWidth)
	}
	for i := range res.NewWorld {
		res.NewWorld[i] = make([]uint8, req.P.ImageWidth)
	}
	copyWhole(g.world, req.World)
	g.completedTurn = 0
	mutex.Unlock()
	if req.P.Turns == 0 {
		copyWhole(res.NewWorld, req.World)
		//res.NewWorld = req.World
		res.Final = gol.FinalTurnComplete{CompletedTurns: req.P.Turns, Alive: calculateAliveCells(g.world)}
		return
	} else {
		workers := []string{"54.227.67.232:8030", "3.90.111.104:8030", "54.237.222.162:8030", "3.87.240.120:8030"}
		clients := make([]*rpc.Client, len(workers))
		for i, worker := range workers {
			client, _ := rpc.Dial("tcp", worker)
			defer client.Close()
			clients[i] = client
		}
		//worker1 := "127.0.0.1:8031"
		//client1, _ := rpc.Dial("tcp", worker1)
		//defer client1.Close()
		//worker2 := "127.0.0.1:8032"
		//client2, _ := rpc.Dial("tcp", worker2)
		//defer client2.Close()
		//worker3 := "127.0.0.1:8033"
		//client3, _ := rpc.Dial("tcp", worker3)
		//defer client3.Close()
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
			fmt.Println("in loop, turn completed:", g.completedTurn)
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
	res.NewWorld = make([][]uint8, op.P.ImageHeight)
	for i := range res.NewWorld {
		res.NewWorld[i] = make([]uint8, op.P.ImageWidth)
	}
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
func main() {
	pAddr := flag.String("port", "8040", "Port to listen on")
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
