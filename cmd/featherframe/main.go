// cmd/featherfinder/main.go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlverezYari/featherframe/internal/config"
	"github.com/AlverezYari/featherframe/internal/headless"
	"github.com/AlverezYari/featherframe/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Load the configuration
	// Determine the config path
	configDir, err := os.UserConfigDir()
	if err != nil {
		fmt.Printf("Error getting user config directory: %v", err)
		os.Exit(1)
	}

	appConfigPath := filepath.Join(configDir, "featherframe")
	configPath := filepath.Join(appConfigPath, "config.json")
	// Load existing config or create a new one
	config, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v", err)
		os.Exit(1)
	}

	// check and launch app in the correct mode TUI/headless

	if config.Headless {
		fmt.Println("Starting app in headless mode!")
		if err := headless.NewHeadless(configPath, config); err != nil {
			fmt.Printf("Headless mode error: %v", err)
			os.Exit(1)
		}
	} else {
		// start app in tui mode

		p := tea.NewProgram(
			tui.New(configPath, config),
			tea.WithAltScreen(),
			tea.WithMouseCellMotion(),
		)

		if _, err := p.Run(); err != nil {
			fmt.Printf("Error running program: %v", err)
			os.Exit(1)
		}
	}
}
