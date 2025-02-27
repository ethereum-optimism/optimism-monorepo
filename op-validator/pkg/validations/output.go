package validations

import (
	"bytes"

	"github.com/olekukonko/tablewriter"
)

type Output struct {
	Errors []string
}

func (o *Output) AsMarkdown() string {
	buf := new(bytes.Buffer)
	table := tablewriter.NewWriter(buf)
	table.SetHeader([]string{"Error", "Description"})
	table.SetAutoMergeCells(true)
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")

	if len(o.Errors) == 0 {
		table.Append([]string{"No errors.", "No errors."})
		table.Render()
		return buf.String()
	}

	for _, error := range o.Errors {
		errDesc := ErrorDescription(error)
		if errDesc == "" {
			errDesc = "Unknown error code, please check the implementation for more details."
		}
		table.Append([]string{error, errDesc})
	}

	table.Render()
	return buf.String()
}
