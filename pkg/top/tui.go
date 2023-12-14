package top

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
	"github.com/kubearmor/kubearmor-client/k8s"

	tea "github.com/charmbracelet/bubbletea"
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("99"))

var k8sClientGlobal *k8s.Client

type model struct {
	table           table.Model
	quitting        bool
	errMsg          error
	updateFrequency time.Duration
}

func NewModel(updateFrequency time.Duration) model {
	t := table.New(table.WithColumns([]table.Column{
		{Title: "Pod", Width: 40},
		{Title: "Container", Width: 20},
		{Title: "Status", Width: 10},
		{Title: "Restarts", Width: 10},
		{Title: "CPU (use/lim)", Width: 15},
		{Title: "Mem (use/lim)", Width: 15},
		{Title: "Age", Width: 10},
		{Title: "QoS", Width: 10},
	}))

	s := table.DefaultStyles()
	s.Header = s.Header.
		Bold(true).
		Italic(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("97")).
		BorderBottom(true)

	s.Selected = lipgloss.NewStyle()
	t.SetStyles(s)
	t.Blur()
	return model{table: t, updateFrequency: updateFrequency}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
	case tickMsg:
		podInfos, err := fetchPodMetrics()
		if err != nil {
			m.errMsg = err
		} else {
			if len(podInfos) > 0 {
				var rows []table.Row
				for _, pi := range podInfos {
					for _, cm := range pi.Metrics {
						rows = append(rows, table.Row{
							pi.Name,
							cm.Name,
							pi.Status,
							fmt.Sprintf("%v", cm.Restarts),
							fmt.Sprintf("%vm/%vm", cm.CPU, cm.CPULimit),
							fmt.Sprintf("%vMi/%vMi", cm.Memory, cm.MemoryLimit),
							pi.Age,
							pi.QoSClass,
						})
					}
				}
				m.table.SetRows(rows)
				dynamicHeight := calculateDynamicHeight(len(rows), 10)
				m.table.SetHeight(dynamicHeight + 1)
			}
		}
		return m, tickEveryInterval(m.updateFrequency)
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) View() string {
	return baseStyle.Render(m.table.View())
}

func runRealTimeTop(opts Options) error {
	updateFrequency := time.Duration(opts.RealTimeUpdateInterval) * time.Second
	m := NewModel(updateFrequency)
	p := tea.NewProgram(&m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running bubble tea: %v", err)
	}
	return nil
}

func tickEveryInterval(interval time.Duration) tea.Cmd {
	return tea.Every(interval, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

type tickMsg struct{}

func (m model) Init() tea.Cmd {
	return tickEveryInterval(m.updateFrequency)
}

func calculateDynamicHeight(numRows, maxHeight int) int {
	if numRows > maxHeight {
		return maxHeight
	}
	return numRows
}
