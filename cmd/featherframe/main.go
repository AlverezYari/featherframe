// cmd/featherfinder/main.go
package main

import (
	"fmt"
	"os"

	"github.com/AlverezYari/featherframe/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	p := tea.NewProgram(
		tui.New(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}
