package utils

import "strings"

type Tabbed struct {
	Rows [][]string
}

func (t *Tabbed) AddRow(cells ...string) *Tabbed {
	t.Rows = append(t.Rows, cells)
	return t
}

func (t *Tabbed) Tabulate() *Tabbed {
	var lengths []int
	for _, row := range t.Rows {
		for col, cell := range row {
			if len(lengths) < col+1 {
				lengths = append(lengths, len(cell))
			} else if lengths[col] < len(cell) {
				lengths[col] = len(cell)
			}
		}
	}

	for r, row := range t.Rows {
		for c, cell := range row {
			extend := lengths[c] - len(cell)
			if extend > 0 {
				t.Rows[r][c] = cell + strings.Repeat(" ", extend)
			}
		}
	}

	return t
}

func (t *Tabbed) Lines() []string {
	var res []string
	for _, row := range t.Rows {
		res = append(res, strings.Join(row, " "))
	}
	return res
}

func (t *Tabbed) String() string {
	var res string
	for _, row := range t.Rows {
		res += strings.Join(row, " ") + "\n"
	}
	return res
}
