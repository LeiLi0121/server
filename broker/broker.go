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
	needKill      chan bool
}

var mutex sync.Mutex

func (g *GolOp) ExecuteTurns(req gol.Request, res *gol.Response) (err error) {
	mutex.Lock()
	g.needSave = make(chan bool, 1)
	g.pause = make(chan bool, 1)
	g.needKill = make(chan bool, 1)
	g.world = initWorld(req.P.ImageHeight, req.P.ImageWidth)
	res.NewWorld = initWorld(req.P.ImageHeight, req.P.ImageWidth)
	copyWhole(g.world, req.World)
	g.completedTurn = 0
	mutex.Unlock()
	if req.P.Turns == 0 {
		copyWhole(res.NewWorld, req.World)
		res.Final = gol.FinalTurnComplete{CompletedTurns: req.P.Turns, Alive: calculateAliveCells(g.world)}
		return
	} else {
		addrs := []string{"54.82.26.158:8030", "54.83.98.138:8030", "54.163.65.222:8030", "3.91.185.189:8030"}
		//addrs := []string{"127.0.0.1:8031", "127.0.0.1:8032", "127.0.0.1:8033", "127.0.0.1:8034"}
		clients := make([]*rpc.Client, len(addrs))
		for i, addr := range addrs {
			client, _ := rpc.Dial("tcp", addr)
			defer client.Close()
			clients[i] = client
		}
		partHeight := req.P.ImageHeight / len(clients)
		haloWorker := make([]worker, 4)
		worker1 := worker{
			worldPart: initWorld(partHeight, req.P.ImageWidth),
			upperHalo: make(chan []uint8, 1),
			lowerHalo: make(chan []uint8, 1),
		}
		worker1.upperHalo <- g.world[req.P.ImageHeight-1]
		worker1.lowerHalo <- g.world[partHeight]
		for i := 0; i < partHeight; i++ {
			copy(worker1.worldPart[i], g.world[i])
		}
		haloWorker[0] = worker1
		worker2 := worker{
			worldPart: initWorld(partHeight, req.P.ImageWidth),
			upperHalo: make(chan []uint8, 1),
			lowerHalo: make(chan []uint8, 1),
		}
		worker2.upperHalo <- g.world[partHeight-1]
		worker2.lowerHalo <- g.world[2*partHeight]
		for i := partHeight; i < 2*partHeight; i++ {
			copy(worker2.worldPart[i-partHeight], g.world[i])
		}
		haloWorker[1] = worker2
		worker3 := worker{
			worldPart: initWorld(partHeight, req.P.ImageWidth),
			upperHalo: make(chan []uint8, 1),
			lowerHalo: make(chan []uint8, 1),
		}
		worker3.upperHalo <- g.world[2*partHeight-1]
		worker3.lowerHalo <- g.world[3*partHeight]
		for i := 2 * partHeight; i < 3*partHeight; i++ {
			copy(worker3.worldPart[i-2*partHeight], g.world[i])
		}
		haloWorker[2] = worker3
		worker4 := worker{
			worldPart: initWorld(partHeight, req.P.ImageWidth),
			upperHalo: make(chan []uint8, 1),
			lowerHalo: make(chan []uint8, 1),
		}
		worker4.upperHalo <- g.world[3*partHeight-1]
		worker4.lowerHalo <- g.world[0]
		for i := 3 * partHeight; i < 4*partHeight; i++ {
			copy(worker4.worldPart[i-3*partHeight], g.world[i])
		}
		haloWorker[3] = worker4
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
			case <-g.needKill:
				for _, client := range clients {
					client.Call(gol.WorkerKill, new(gol.KeyPress), new(rpc.Response))
				}
				os.Exit(0)

			default:
				break

			}
			allPartsChan := make(chan [][]uint8, 1)
			if t == 1 {
				worker1.worldPart = nil
				worker2.worldPart = nil
				worker3.worldPart = nil
				worker4.worldPart = nil
			}

			for i := 0; i < len(clients); i++ {
				go func(i int) {
					request := gol.HaloReq{nil, <-haloWorker[i].upperHalo,
						<-haloWorker[i].lowerHalo, partHeight, req.P.ImageWidth}
					if t == 0 {
						request.WorldPart = haloWorker[i].worldPart
					}
					response := new(gol.HaloRes)
					err := clients[i].Call(gol.WorkerProcess, request, response)
					if err != nil {
						return
					}
					mutex.Lock()
					for h := i * partHeight; h < (i+1)*partHeight; h++ {
						copy(res.NewWorld[h], response.WorldPart[h-i*partHeight])
					}
					if i == 0 { //1
						worker2.upperHalo <- response.WorldPart[partHeight-1]
						worker4.lowerHalo <- response.WorldPart[0]
					} else if i == 1 { //2
						worker1.lowerHalo <- response.WorldPart[0]
						worker3.upperHalo <- response.WorldPart[partHeight-1]
					} else if i == 2 {
						worker2.lowerHalo <- response.WorldPart[0]
						worker4.upperHalo <- response.WorldPart[partHeight-1]

					} else {
						worker1.upperHalo <- response.WorldPart[partHeight-1]
						worker3.lowerHalo <- response.WorldPart[0]
					}
					allPartsChan <- res.NewWorld
					mutex.Unlock()
				}(i)
			}
			for i := 0; i < len(clients); i++ {
				<-allPartsChan
			}
			mutex.Lock()
			copyWhole(g.world, res.NewWorld)
			g.completedTurn = t + 1
			mutex.Unlock()
		}

		res.Final = gol.FinalTurnComplete{CompletedTurns: req.P.Turns, Alive: calculateAliveCells(res.NewWorld)}
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
	g.needKill <- true
	//os.Exit(0)
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
	upperHalo chan []uint8
	lowerHalo chan []uint8
}
