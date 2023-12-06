package main

import (
	"SOMAS2023/internal/server"
	"fmt"
)

func main() {
	fmt.Println("Hello Agents")
	s := server.Initialize(1)
	s.UpdateGameStates()
	// i := 0
	// for _, agent := range s.GetAgentMap() {
	// 	if i < 3 {
	// 		agent.UpdateColour(utils.Red)
	// 	}
	// 	i++
	// }
	s.Start()
}
