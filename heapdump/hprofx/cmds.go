// cmds.go — the analyses. Each writes into an *out so the same code produces
// terminal tables or the markdown report.
package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ---------------------------------------------------------------- summary

func cmdSummary(o *out, g *Graph) {
	m := g.M
	var shallow int64
	counts := [4]int64{}
	for i := 0; i < g.N(); i++ {
		shallow += int64(g.Size[i])
		counts[g.Kind[i]]++
	}
	o.heading(2, "Dump summary")
	rows := [][]string{
		{"File", m.Path},
		{"File size", fmt.Sprintf("%.2f MB", mb(m.FileSize))},
		{"Dump time", time.UnixMilli(int64(m.Timestamp)).UTC().Format("2006-01-02 15:04:05 UTC")},
		{"Live bytes (shallow total)", fmt.Sprintf("%.2f MB", mb(shallow))},
		{"Objects", fmt.Sprintf("%d", g.N())},
		{"  instances", fmt.Sprintf("%d", counts[kindInstance])},
		{"  object arrays", fmt.Sprintf("%d", counts[kindObjArray])},
		{"  primitive arrays", fmt.Sprintf("%d", counts[kindPrimArray])},
		{"Classes", fmt.Sprintf("%d", counts[kindClass])},
		{"Threads", fmt.Sprintf("%d", len(m.Threads))},
	}
	kinds := make([]string, 0, len(m.RootKinds))
	for k, v := range m.RootKinds {
		kinds = append(kinds, fmt.Sprintf("%s=%d", k, v))
	}
	sort.Strings(kinds)
	rows = append(rows, []string{"GC roots", strings.Join(kinds, " ")})
	o.table([]string{"Property", "Value"}, "ll", rows)
}

// ---------------------------------------------------------------- histogram

func cmdHisto(o *out, g *Graph, top int, byCount bool) {
	stats := map[string]*tally{}
	var total int64
	for i := 0; i < g.N(); i++ {
		if g.Kind[i] == kindClass {
			continue
		}
		add(stats, g.Name(int32(i)), int64(g.Size[i]))
		total += int64(g.Size[i])
	}
	rows := sortTally(stats, !byCount)
	title := "Top classes by shallow size"
	if byCount {
		title = "Top classes by instance count"
	}
	o.heading(2, title)
	var out [][]string
	for i, r := range rows {
		if i >= top {
			break
		}
		out = append(out, []string{
			truncate(r.Key, 74),
			fmt.Sprintf("%.2f", mb(r.Bytes)),
			fmt.Sprintf("%d", r.Count),
			fmt.Sprintf("%.2f%%", 100*float64(r.Bytes)/float64(total)),
		})
	}
	o.table([]string{"Class", "Shallow MB", "Count", "% heap"}, "lrrr", out)
}

// ---------------------------------------------------------------- holders

// cmdHolders answers "who is holding all of these?" — the single most useful
// question when one type dominates the histogram. It groups every instance of
// the target type by the class of the object that references it.
func cmdHolders(o *out, g *Graph, pattern string, top int) {
	re := mustCompile(pattern)
	holders := map[string]*tally{}
	var matched, matchedBytes int64
	for i := 0; i < g.N(); i++ {
		if !re.MatchString(g.Name(int32(i))) {
			continue
		}
		matched++
		matchedBytes += int64(g.Size[i])
		refs := g.Referrers(int32(i))
		key := "<no referrer — unreachable or a GC root>"
		if len(refs) > 0 {
			key = g.Name(refs[0])
			if len(refs) > 1 {
				key += fmt.Sprintf(" (+%d other referrers)", len(refs)-1)
			}
		}
		add(holders, key, int64(g.Size[i]))
	}
	o.heading(2, fmt.Sprintf("Who holds %s", pattern))
	o.para(fmt.Sprintf("%d matching objects, %.2f MB shallow, grouped by the class of their first referrer.",
		matched, mb(matchedBytes)))
	var rows [][]string
	for i, r := range sortTally(holders, true) {
		if i >= top {
			break
		}
		rows = append(rows, []string{
			truncate(r.Key, 74),
			fmt.Sprintf("%.2f", mb(r.Bytes)),
			fmt.Sprintf("%d", r.Count),
		})
	}
	o.table([]string{"Referrer", "MB", "Count"}, "lrr", rows)
}

// ---------------------------------------------------------------- retained

func cmdRetained(o *out, g *Graph, d *Dom, top int, minMB float64) {
	type row struct {
		i int32
		r int64
	}
	minBytes := int64(minMB * (1 << 20))
	var tops []row
	for i := 0; i < g.N(); i++ {
		if d.Retained[i] >= minBytes {
			tops = append(tops, row{int32(i), d.Retained[i]})
		}
	}
	sort.Slice(tops, func(a, b int) bool { return tops[a].r > tops[b].r })

	o.heading(2, "Largest objects by retained size")
	o.para(fmt.Sprintf("Retained size is the memory freed if the object became unreachable. Unreachable garbage in this dump: %.2f MB.", mb(g.Unreach)))

	// The dominator tree of a chain (a linked list, say) nests every node inside
	// its predecessor, so print the chain once rather than one row per link.
	shown := 0
	var b strings.Builder
	seen := map[int32]bool{}
	for _, t := range tops {
		if shown >= top {
			break
		}
		if seen[t.i] {
			continue
		}
		chain := d.DomChain(t.i, 8)
		// Suppress a node whose dominator retains nearly the same bytes: it is
		// just the next link down the same chain.
		if p := d.Idom[t.i]; p >= 0 && p != d.Root && float64(d.Retained[p]-t.r) < 0.02*float64(t.r) {
			continue
		}
		for _, c := range chain {
			seen[c] = true
		}
		fmt.Fprintf(&b, "%-58s  retained %9.2f MB  shallow %d B\n", truncate(g.Label(t.i), 58), mb(t.r), g.Size[t.i])
		if len(chain) > 0 {
			names := make([]string, 0, len(chain))
			for _, c := range chain {
				names = append(names, truncate(g.Name(c), 44))
			}
			fmt.Fprintf(&b, "    dominated by: %s\n", strings.Join(names, " <- "))
		}
		shown++
	}
	o.code(b.String())

	byClass := map[string]*tally{}
	for i := 0; i < g.N(); i++ {
		if d.Reachable(int32(i)) {
			add(byClass, g.Name(int32(i)), d.Retained[i])
		}
	}
	o.heading(3, "Retained bytes summed per class")
	o.para("Nested and therefore **not additive** — a linked list's nodes each dominate the rest of the list. Useful for ranking, not for totals.")
	var rows [][]string
	for i, r := range sortTally(byClass, true) {
		if i >= top {
			break
		}
		rows = append(rows, []string{
			truncate(r.Key, 70),
			fmt.Sprintf("%.2f", mb(r.Bytes)),
			fmt.Sprintf("%d", r.Count),
		})
	}
	o.table([]string{"Class", "Retained MB", "Instances"}, "lrr", rows)
}

// ---------------------------------------------------------------- threads

func cmdThreads(o *out, g *Graph, maxFrames int) {
	m := g.M
	type group struct {
		names  []string
		frames []string
		status string
	}
	groups := map[string]*group{}
	var order []string
	for _, t := range m.Threads {
		name, status := "<unknown>", ""
		if i := g.Lookup(t.ObjID); i >= 0 {
			for _, f := range g.Fields(i) {
				switch f.Name {
				case "name":
					if s, ok := g.StringOf(g.Lookup(f.Raw)); ok {
						name = s
					}
				case "threadStatus":
					status = threadStatus(f.Raw)
				}
			}
		}
		var frames []string
		for _, fid := range m.Traces[t.StackSerial] {
			frames = append(frames, m.Frames[fid])
		}
		key := status + "\x00" + strings.Join(frames, "\n")
		gr := groups[key]
		if gr == nil {
			gr = &group{frames: frames, status: status}
			groups[key] = gr
			order = append(order, key)
		}
		gr.names = append(gr.names, name)
	}
	sort.SliceStable(order, func(a, b int) bool {
		return len(groups[order[a]].names) > len(groups[order[b]].names)
	})

	o.heading(2, fmt.Sprintf("Threads (%d, grouped by identical stack)", len(m.Threads)))
	var b strings.Builder
	for _, k := range order {
		gr := groups[k]
		names := gr.names
		if len(names) > 6 {
			names = append(append([]string{}, names[:6]...), fmt.Sprintf("… +%d more", len(gr.names)-6))
		}
		fmt.Fprintf(&b, "\n×%d  [%s]  %s\n", len(gr.names), gr.status, strings.Join(names, ", "))
		for i, f := range gr.frames {
			if i >= maxFrames {
				fmt.Fprintf(&b, "        … %d more frames\n", len(gr.frames)-maxFrames)
				break
			}
			fmt.Fprintf(&b, "        at %s\n", f)
		}
	}
	o.code(strings.TrimLeft(b.String(), "\n"))
}

// threadStatus decodes java.lang.Thread.threadStatus bit flags.
func threadStatus(s uint64) string {
	flags := []struct {
		bit  uint64
		name string
	}{
		{1, "ALIVE"}, {2, "TERMINATED"}, {4, "RUNNABLE"}, {8, "BLOCKED_ON_MONITOR"},
		{16, "WAITING_INDEFINITELY"}, {32, "WAITING_TIMED"}, {64, "SLEEPING"},
		{128, "IN_OBJECT_WAIT"}, {1024, "PARKED"}, {4096, "SUSPENDED"}, {8192, "NATIVE"},
	}
	var out []string
	for _, f := range flags {
		if s&f.bit != 0 {
			out = append(out, f.name)
		}
	}
	if len(out) == 0 {
		return "UNKNOWN"
	}
	return strings.Join(out, "|")
}

// ---------------------------------------------------------------- inspect

func cmdInspect(o *out, g *Graph, pattern string, limit int, fieldFilter string) {
	re := mustCompile(pattern)
	var ff *regexp.Regexp
	if fieldFilter != "" {
		ff = mustCompile(fieldFilter)
	}
	o.heading(2, fmt.Sprintf("Instances matching %s", pattern))
	var b strings.Builder
	shown, total := 0, 0
	for i := 0; i < g.N(); i++ {
		if g.Kind[i] != kindInstance || !re.MatchString(g.Name(int32(i))) {
			continue
		}
		total++
		if shown >= limit {
			continue
		}
		shown++
		fmt.Fprintf(&b, "\n%s\n", g.Label(int32(i)))
		for _, f := range g.Fields(int32(i)) {
			if ff != nil && !ff.MatchString(f.Name) {
				continue
			}
			fmt.Fprintf(&b, "    %-28s %s\n", f.Name, truncate(g.Render(f), 150))
		}
	}
	if total > shown {
		fmt.Fprintf(&b, "\n… %d more instances not shown (raise -limit)\n", total-shown)
	}
	if total == 0 {
		o.para("_no matching instances_")
		return
	}
	o.code(strings.TrimLeft(b.String(), "\n"))
}

// ---------------------------------------------------------------- paths

// cmdPaths prints a shortest reference path from a GC root, which is what tells
// you why an object is still alive.
func cmdPaths(o *out, g *Graph, d *Dom, pattern string, limit int) {
	re := mustCompile(pattern)
	parent := bfsFromRoots(g, d)

	o.heading(2, fmt.Sprintf("Paths from GC roots to %s", pattern))
	var b strings.Builder
	shown := 0
	for i := 0; i < g.N() && shown < limit; i++ {
		if !re.MatchString(g.Name(int32(i))) || !d.Reachable(int32(i)) {
			continue
		}
		shown++
		var chain []int32
		for v := int32(i); v >= 0 && v != d.Root; v = parent[v] {
			chain = append(chain, v)
			if len(chain) > 200 {
				break
			}
		}
		fmt.Fprintf(&b, "\n%s  (retained %.2f MB)\n", g.Label(int32(i)), mb(d.Retained[i]))
		for k := len(chain) - 1; k >= 0; k-- {
			v := chain[k]
			label := ""
			if k < len(chain)-1 {
				label = g.EdgeLabel(chain[k+1], v)
			} else if g.RootSet[v] {
				label = " (GC root)"
			}
			fmt.Fprintf(&b, "  %s%s%s\n", strings.Repeat("  ", len(chain)-1-k), truncate(g.Label(v), 96), label)
		}
	}
	if shown == 0 {
		o.para("_no reachable matching objects_")
		return
	}
	o.code(strings.TrimLeft(b.String(), "\n"))
}

func bfsFromRoots(g *Graph, d *Dom) []int32 {
	n := g.N()
	parent := make([]int32, n)
	for i := range parent {
		parent[i] = -2 // unvisited
	}
	queue := make([]int32, 0, 1<<16)
	for _, r := range g.Roots {
		parent[r] = d.Root
		queue = append(queue, r)
	}
	for head := 0; head < len(queue); head++ {
		v := queue[head]
		for _, w := range g.Refs(v) {
			if parent[w] == -2 {
				parent[w] = v
				queue = append(queue, w)
			}
		}
	}
	for i := range parent {
		if parent[i] == -2 {
			parent[i] = -1
		}
	}
	return parent
}

func mustCompile(p string) *regexp.Regexp {
	re, err := regexp.Compile(p)
	if err != nil {
		fatal("bad pattern %q: %v", p, err)
	}
	return re
}
