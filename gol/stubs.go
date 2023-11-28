package gol

var ExecuteTurns = "GolOp.ExecuteTurns"
var ExecuteTimer = "GolOp.Timer"
var ExecuteKey = "GolOp.KeyOp"
var KillProcess = "GolOp.Kill"
var ResumeProcess = "GolOp.Resume"

var WorkerProcess = "Worker.Worker"

type Request struct {
	World [][]uint8
	P     Params
}

type Response struct {
	NewWorld    [][]uint8
	Final       FinalTurnComplete
	CurrentTurn int
}

type KeyPress struct {
	Key rune
	P   Params
}
type ReportAlive struct {
	Alive AliveCellsCount
}

type PartInfo struct {
	World  [][]uint8
	StartY int
	EndY   int
	Width  int
}

type NewPart struct {
	NewWorldPart [][]uint8
}

type HaloReq struct {
	WorldPart  [][]uint8
	UpperHalo  []uint8
	LowerHalo  []uint8
	PartHeight int
	Width      int
}

type HaloRes struct {
	WorldPart [][]uint8
}
