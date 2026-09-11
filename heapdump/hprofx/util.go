package main

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
)

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "hprofx: "+format+"\n", args...)
	os.Exit(1)
}

func logf(verbose bool, format string, args ...any) {
	if verbose {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}

func mb(b int64) float64 { return float64(b) / (1 << 20) }

func truncate(s string, n int) string {
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		return r
	}, s)
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}

func float32frombits(b uint32) float32 { return math.Float32frombits(b) }
func float64frombits(b uint64) float64 { return math.Float64frombits(b) }

// tally accumulates a count and a byte total per key.
type tally struct {
	Count int64
	Bytes int64
}

type tallyRow struct {
	Key string
	tally
}

func sortTally(m map[string]*tally, byBytes bool) []tallyRow {
	rows := make([]tallyRow, 0, len(m))
	for k, v := range m {
		rows = append(rows, tallyRow{k, *v})
	}
	sort.Slice(rows, func(i, j int) bool {
		if byBytes {
			if rows[i].Bytes != rows[j].Bytes {
				return rows[i].Bytes > rows[j].Bytes
			}
			return rows[i].Count > rows[j].Count
		}
		if rows[i].Count != rows[j].Count {
			return rows[i].Count > rows[j].Count
		}
		return rows[i].Bytes > rows[j].Bytes
	})
	return rows
}

func add(m map[string]*tally, key string, bytes int64) {
	t := m[key]
	if t == nil {
		t = &tally{}
		m[key] = t
	}
	t.Count++
	t.Bytes += bytes
}

// out is a tiny writer that renders either plain text or markdown tables.
type out struct {
	w        *strings.Builder
	markdown bool
}

func newOut(markdown bool) *out { return &out{w: &strings.Builder{}, markdown: markdown} }

func (o *out) String() string { return o.w.String() }

func (o *out) printf(format string, args ...any) { fmt.Fprintf(o.w, format, args...) }

func (o *out) heading(level int, text string) {
	if o.markdown {
		o.printf("\n%s %s\n\n", strings.Repeat("#", level), text)
		return
	}
	o.printf("\n=== %s ===\n", strings.ToUpper(text))
}

func (o *out) para(text string) {
	if o.markdown {
		o.printf("%s\n\n", text)
		return
	}
	o.printf("%s\n", text)
}

// table renders rows with the given headers. align is a string of 'l'/'r'.
func (o *out) table(headers []string, align string, rows [][]string) {
	if len(rows) == 0 {
		o.para("_(none)_")
		return
	}
	if o.markdown {
		o.printf("| %s |\n", strings.Join(headers, " | "))
		seps := make([]string, len(headers))
		for i := range headers {
			if i < len(align) && align[i] == 'r' {
				seps[i] = "---:"
			} else {
				seps[i] = "---"
			}
		}
		o.printf("| %s |\n", strings.Join(seps, " | "))
		for _, r := range rows {
			cells := make([]string, len(r))
			for i, c := range r {
				cells[i] = strings.ReplaceAll(c, "|", "\\|")
			}
			o.printf("| %s |\n", strings.Join(cells, " | "))
		}
		o.printf("\n")
		return
	}
	width := make([]int, len(headers))
	for i, h := range headers {
		width[i] = len([]rune(h))
	}
	for _, r := range rows {
		for i, c := range r {
			if n := len([]rune(c)); n > width[i] {
				width[i] = n
			}
		}
	}
	render := func(cells []string) {
		parts := make([]string, len(cells))
		for i, c := range cells {
			pad := width[i] - len([]rune(c))
			if i < len(align) && align[i] == 'r' {
				parts[i] = strings.Repeat(" ", pad) + c
			} else {
				parts[i] = c + strings.Repeat(" ", pad)
			}
		}
		o.printf("%s\n", strings.TrimRight(strings.Join(parts, "  "), " "))
	}
	render(headers)
	seps := make([]string, len(headers))
	for i := range headers {
		seps[i] = strings.Repeat("-", width[i])
	}
	render(seps)
	for _, r := range rows {
		render(r)
	}
}

func (o *out) code(text string) {
	if o.markdown {
		o.printf("```\n%s\n```\n\n", strings.TrimRight(text, "\n"))
		return
	}
	o.printf("%s\n", text)
}
