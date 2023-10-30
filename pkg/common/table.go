package common

import (
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/table"
	"github.com/jedib0t/go-pretty/text"
	"github.com/olekukonko/tablewriter"
)

func tableOutput(header []string, data [][]string, title string) {
	tr := table.NewWriter()

	var row table.Row
	for _, d := range header {
		row = append(row, d)
	}
	tr.AppendHeader(row)
	tr.SetAlignHeader([]text.Align{text.AlignCenter})
	for _, v := range data {
		row = table.Row{}
		for _, d := range v {
			row = append(row, d)
		}
		tr.AppendRow(row)
	}
	tr.SetStyle(table.StyleLight)
	tr.SetTitle(title)
	fmt.Printf("%s", tr.Render())
	//fmt.Printf("%s", tr.RenderMarkdown())
	tr.SetTitle("")

}

// WriteTable function
func WriteTable(header []string, data [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	for _, v := range data {
		table.Append(v)
	}
	table.Render()
}
