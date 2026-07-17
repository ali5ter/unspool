package cmd

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/ali5ter/unspool/config"
	"github.com/ali5ter/unspool/internal/playback"
	"github.com/ali5ter/unspool/internal/tui"
)

func runTUI(cfg *config.Config) error {
	if err := playback.CheckDependencies(); err != nil {
		fmt.Fprintln(os.Stderr, "warning:", err)
	}
	p := tea.NewProgram(tui.New(cfg))
	_, err := p.Run()
	return err
}
