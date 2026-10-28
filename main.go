package main

import (
	"fmt"
	"os"

	"github.com/lujiawu/nbalive/backend"
	"github.com/lujiawu/nbalive/ui"
)

func main() {
	games, err := backend.GetGamesForDate(backend.Today())
	if err != nil {
		fmt.Println("Error loading today's games:", err)
		os.Exit(1)
	}

	if err := ui.Run(games); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
