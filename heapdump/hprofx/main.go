// hprofx — a dependency-free analyzer for Java HPROF heap dumps.
//
// It exists because the usual tools often are not available where a dump lands:
// jhat was removed in JDK 9, and Eclipse MAT is a desktop install. This reads
// the binary format directly and answers the questions that actually diagnose a
// leak — what is big, who holds it, why is it still reachable, and what were the
// threads doing.
//
//	hprofx <dump.hprof> <command> [flags]
//
// Start with `report`, then drill in with `holders`, `paths` and `inspect`.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

// version is stamped at build time: -ldflags "-X main.version=..."
var version = "dev"

const usage = `hprofx — Java heap dump analyzer

usage: hprofx <dump.hprof> <command> [flags]

commands:
  summary                 header, object counts, GC roots
  histo                   class histogram        [-top N] [-by count|shallow]
  holders                 who references a type  [-class REGEX] [-top N]
  retained                dominator tree, retained sizes, top consumers
                                                 [-top N] [-min MB]
  threads                 reconstructed thread dump, grouped by stack
                                                 [-frames N]
  amqp                    RabbitMQ backlog: routing keys, consumers, ages,
                          ack modes             [-top N] [-samples N] [-age REGEX]
  inspect                 dump instance fields   -class REGEX [-limit N] [-field REGEX]
  paths                   shortest path from a GC root
                                                 -class REGEX [-limit N]
  report                  run the full analysis into a markdown file  [-o FILE]

global flags:
  -q                      quiet (suppress progress on stderr)

examples:
  hprofx heap.hprof report -o analysis.md
  hprofx heap.hprof holders -class 'byte\[\]'
  hprofx heap.hprof paths -class 'com\.example\.CacheEntry' -limit 3
  hprofx heap.hprof inspect -class 'BlockingQueueConsumer' -field 'prefetch|ackMode|queues'
  hprofx heap.hprof amqp -age '"traceId":"(\d{13})'
`

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "-version" || os.Args[1] == "version") {
		fmt.Printf("hprofx %s\n", version)
		return
	}
	if len(os.Args) < 3 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	path, cmd := os.Args[1], os.Args[2]
	args := os.Args[3:]

	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	var (
		top     = fs.Int("top", 40, "number of rows to show")
		by      = fs.String("by", "shallow", "histogram sort: shallow|count")
		class   = fs.String("class", "", "class name regex")
		limit   = fs.Int("limit", 10, "max objects to print")
		field   = fs.String("field", "", "field name regex (inspect)")
		frames  = fs.Int("frames", 18, "max stack frames per thread group")
		minMB   = fs.Float64("min", 1, "minimum retained MB to report")
		samples = fs.Int("samples", 3, "sample message bodies to print (amqp)")
		ageRe   = fs.String("age", "", "regex over message bodies whose first capture group is epoch millis (amqp)")
		outFile = fs.String("o", "", "write markdown report to this file")
		quiet   = fs.Bool("q", false, "suppress progress output")
	)
	fs.Parse(args)
	verbose := !*quiet

	start := time.Now()
	logf(verbose, "reading metadata")
	meta := LoadMeta(path)
	meta.Timestamp = dumpTimestamp(path)

	needEdges := cmd == "holders" || cmd == "retained" || cmd == "paths" ||
		cmd == "amqp" || cmd == "report"
	g := BuildGraph(meta, verbose, needEdges)

	var d *Dom
	if cmd == "retained" || cmd == "paths" || cmd == "report" {
		d = Dominators(g, verbose)
	}

	o := newOut(cmd == "report")

	switch cmd {
	case "summary":
		cmdSummary(o, g)
	case "histo":
		cmdHisto(o, g, *top, *by == "count")
	case "holders":
		pattern := *class
		if pattern == "" {
			pattern = `^byte\[\]$` // the usual culprit
		}
		cmdHolders(o, g, pattern, *top)
	case "retained":
		cmdRetained(o, g, d, *top, *minMB)
	case "threads":
		cmdThreads(o, g, *frames)
	case "amqp":
		cmdAMQP(o, g, *top, *samples, *ageRe)
	case "inspect":
		if *class == "" {
			fatal("inspect requires -class REGEX")
		}
		cmdInspect(o, g, *class, *limit, *field)
	case "paths":
		if *class == "" {
			fatal("paths requires -class REGEX")
		}
		cmdPaths(o, g, d, *class, *limit)
	case "report":
		buildReport(o, g, d, *top, *frames, *samples, *ageRe)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	text := o.String()
	if cmd == "report" {
		name := *outFile
		if name == "" {
			name = strings.TrimSuffix(path, ".hprof") + "-report.md"
		}
		if err := os.WriteFile(name, []byte(text), 0o644); err != nil {
			fatal("writing %s: %v", name, err)
		}
		fmt.Printf("report written to %s\n", name)
	} else {
		fmt.Print(text)
	}
	logf(verbose, "done in %s", time.Since(start).Round(time.Millisecond))
}

// buildReport runs every analysis in the order you would actually read them.
func buildReport(o *out, g *Graph, d *Dom, top, frames, samples int, ageRe string) {
	o.printf("# Heap dump analysis — %s\n\n", g.M.Path)
	o.para(fmt.Sprintf("Generated by hprofx on %s.", time.Now().Format("2006-01-02 15:04")))
	o.para("Read this top to bottom: what is big, who is holding it, why it is still reachable, and what the threads were doing when the dump was taken.")

	cmdSummary(o, g)
	cmdHisto(o, g, top, false)
	cmdHisto(o, g, top, true)

	// Point the holder analysis at whatever actually dominates the histogram.
	biggest, biggestBytes := "", int64(0)
	stats := map[string]*tally{}
	for i := 0; i < g.N(); i++ {
		if g.Kind[i] == kindClass {
			continue
		}
		add(stats, g.Name(int32(i)), int64(g.Size[i]))
	}
	for _, r := range sortTally(stats, true) {
		biggest, biggestBytes = r.Key, r.Bytes
		break
	}
	if biggest != "" {
		o.para(fmt.Sprintf("`%s` is the largest type at %.2f MB, so the next section attributes it to its holders.", biggest, mb(biggestBytes)))
		cmdHolders(o, g, "^"+regexpEscape(biggest)+"$", top)
	}

	cmdRetained(o, g, d, top, 1)

	// Only worth a section when the dump actually contains buffered deliveries.
	if hasClass(g, envelopeClass) {
		cmdAMQP(o, g, top, samples, ageRe)
	}

	cmdThreads(o, g, frames)

	o.heading(2, "Method notes")
	o.para("Shallow sizes, instance counts, holder attribution and thread stacks are exact per-object statistics. Retained sizes come from a Lengauer-Tarjan dominator tree over the full reference graph rooted at a virtual node joined to every GC root; they are exact for the graph as recorded, but a dump only records strong references, so objects kept alive solely through soft/weak/phantom references appear reachable here.")
}

func hasClass(g *Graph, name string) bool {
	for i := 0; i < g.N(); i++ {
		if g.Kind[i] == kindInstance && g.M.className(g.ClassOf[i]) == name {
			return true
		}
	}
	return false
}

func regexpEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(`\.+*?()|[]{}^$`, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
