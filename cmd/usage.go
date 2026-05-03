package cmd

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/suho-han/one-click-tools/internal/update"
	"github.com/suho-han/one-click-tools/internal/usage"
)

type usageModel struct {
	results      []usage.UsageResult
	err          error
	done         bool
	spinner      int
	orderedTools []update.Tool
	activeIdx    int
}

func (m usageModel) Init() tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			res, err := usage.GetUsage()
			if err != nil {
				return err
			}
			return res
		},
		tea.Tick(400*time.Millisecond, func(t time.Time) tea.Msg {
			return switchProviderMsg{}
		}),
	)
}

func (m usageModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case []usage.UsageResult:
		m.results = msg
		m.done = true
		return m, tea.Quit
	case error:
		m.err = msg
		m.done = true
		return m, tea.Quit
	case spinnerMsg:
		m.spinner++
		return m, tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
			return spinnerMsg{}
		})
	case switchProviderMsg:
		if len(m.orderedTools) > 0 {
			m.activeIdx = (m.activeIdx + 1) % len(m.orderedTools)
		}
		return m, tea.Tick(400*time.Millisecond, func(t time.Time) tea.Msg {
			return switchProviderMsg{}
		})
	}
	return m, nil
}

type spinnerMsg struct{}
type switchProviderMsg struct{}

func (m usageModel) View() string {
	if m.done {
		return ""
	}
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	s := spinners[m.spinner%len(spinners)]

	target := "AI providers"
	if len(m.orderedTools) > 0 {
		t := m.orderedTools[m.activeIdx]
		target = t.Colorize(t.Name)
	}

	return fmt.Sprintf("\n  %s Fetching usage data from %s...\n", s, target)
}

var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Show tool usage report",
	Long: `Show tool usage report for configured AI developer tools.

To properly fetch usage, ensure you are authenticated:
  - Gemini:  Run 'gemini auth' to log in via browser
  - Claude:  Run 'claude auth login' to log in via browser
  - Copilot: Configure your token via 'oct config'
  - Codex:   Automatically reads from local session logs`,
	Run: func(cmd *cobra.Command, args []string) {
		jsonMode, _ := cmd.Flags().GetBool("json")
		
		if jsonMode {
			// For JSON, we might not want a spinner if it goes to stdout/pipe
			results, err := usage.GetUsage()
			if err != nil {
				fmt.Printf("Error fetching usage: %v\n", err)
				return
			}
			usage.PrintJSON(results)
			return
		}

		order := viper.GetStringSlice("agent_order")
		if len(order) == 0 {
			order = []string{"gemini", "claude", "copilot", "codex"}
		}
		orderedTools := update.GetOrderedTools(order)

		m := usageModel{
			orderedTools: orderedTools,
		}
		p := tea.NewProgram(m)
		
		// Start spinner tick
		go func() {
			time.Sleep(100 * time.Millisecond)
			p.Send(spinnerMsg{})
		}()

		finalModel, err := p.Run()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fm := finalModel.(usageModel)
		if fm.err != nil {
			fmt.Printf("Error fetching usage: %v\n", fm.err)
			return
		}

		if len(fm.results) > 0 {
			usage.PrintTable(fm.results)
			fmt.Println("\nTip: Run 'oct usage --help' for authentication instructions.")
		}
	},
}

func init() {
	rootCmd.AddCommand(usageCmd)
	usageCmd.Flags().Bool("json", false, "Output in JSON format")
}
