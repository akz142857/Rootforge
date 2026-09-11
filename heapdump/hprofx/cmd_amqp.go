// cmd_amqp.go — RabbitMQ / Spring AMQP specifics.
//
// A message backlog held in the client is a recurring and easily misread OOM
// shape: the histogram just says byte[], and the config that allowed it lives in
// object fields rather than anywhere in the stack traces. This command pulls out
// the whole picture — what is queued, for which routing key and consumer, how
// big, how old, and the consumer settings that let it accumulate.
//
// Deliveries are found structurally rather than by hardcoded class names: any
// object holding a com.rabbitmq.client.Envelope is a delivery, which covers the
// client's own ConsumerDispatcher runnables, Spring AMQP's Delivery, and
// whatever the next version renames them to.
package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	envelopeClass = "com.rabbitmq.client.Envelope"
	propsClass    = "com.rabbitmq.client.AMQP$BasicProperties"
)

// deliveryShape records which field of a holder class carries which part of a
// delivery. Discovered once per class from a sample instance.
type deliveryShape struct {
	tag, envelope, props, body string
	ok                         bool
}

type delivery struct {
	node     int32
	holder   string
	tag      string
	exchange string
	routing  string
	bodyLen  int64
	bodyNode int32
	stamp    int64 // epoch millis, 0 when unknown
}

func cmdAMQP(o *out, g *Graph, top, samples int, ageRegex string) {
	// Envelopes are the anchor: cheap to enumerate, and every delivery has one.
	var envelopes []int32
	for i := 0; i < g.N(); i++ {
		if g.Kind[i] == kindInstance && g.M.className(g.ClassOf[i]) == envelopeClass {
			envelopes = append(envelopes, int32(i))
		}
	}

	o.heading(2, "AMQP delivery backlog")
	if len(envelopes) == 0 {
		o.para("_no com.rabbitmq.client.Envelope instances — this dump has no buffered AMQP deliveries_")
		return
	}

	shapes := map[uint64]*deliveryShape{}
	seen := make(map[int32]bool, len(envelopes))
	var deliveries []delivery

	for _, env := range envelopes {
		for _, holder := range g.Referrers(env) {
			if seen[holder] || g.Kind[holder] != kindInstance {
				continue
			}
			seen[holder] = true
			shape := g.deliveryShapeOf(holder, shapes)
			if !shape.ok {
				continue
			}
			deliveries = append(deliveries, g.readDelivery(holder, env, shape))
		}
	}
	if len(deliveries) == 0 {
		o.para("_envelopes present but no object holds one together with a body — nothing to report_")
		return
	}

	var totalBytes int64
	byRoute := map[string]*tally{}
	byTag := map[string]*tally{}
	byHolder := map[string]*tally{}
	sizes := make([]int64, 0, len(deliveries))
	stamps := make([]int64, 0, len(deliveries))
	for _, d := range deliveries {
		totalBytes += d.bodyLen
		sizes = append(sizes, d.bodyLen)
		add(byRoute, d.exchange+" / "+d.routing, d.bodyLen)
		add(byTag, d.tag, d.bodyLen)
		add(byHolder, d.holder, d.bodyLen)
		if d.stamp > 0 {
			stamps = append(stamps, d.stamp)
		}
	}

	o.para(fmt.Sprintf("**%d buffered deliveries holding %.2f MB of message bodies** (mean %.1f KB).",
		len(deliveries), mb(totalBytes), float64(totalBytes)/float64(len(deliveries))/1024))

	o.heading(3, "Where the deliveries are buffered")
	o.table([]string{"Holder", "MB", "Messages"}, "lrr", tallyRows(byHolder, top))

	o.heading(3, "By exchange / routing key")
	o.table([]string{"exchange / routingKey", "MB", "Messages"}, "lrr", tallyRows(byRoute, top))

	o.heading(3, "By consumer tag")
	o.table([]string{"consumerTag", "MB", "Messages"}, "lrr", tallyRows(byTag, top))

	sort.Slice(sizes, func(i, j int) bool { return sizes[i] < sizes[j] })
	o.heading(3, "Message body size")
	o.table([]string{"min", "p50", "p90", "p99", "max"}, "rrrrr", [][]string{{
		bytesLabel(sizes[0]), bytesLabel(pct(sizes, .50)), bytesLabel(pct(sizes, .90)),
		bytesLabel(pct(sizes, .99)), bytesLabel(sizes[len(sizes)-1]),
	}})

	g.reportBacklogAge(o, deliveries, stamps, ageRegex)
	g.reportConsumers(o)
	g.reportSamples(o, deliveries, samples)
}

// deliveryShapeOf works out, once per holder class, which fields carry the
// consumer tag, envelope, properties and body.
func (g *Graph) deliveryShapeOf(node int32, cache map[uint64]*deliveryShape) *deliveryShape {
	cid := g.ClassOf[node]
	if s, ok := cache[cid]; ok {
		return s
	}
	s := &deliveryShape{}
	for _, f := range g.Fields(node) {
		if f.Type != 2 || f.Raw == 0 {
			continue
		}
		t := g.Lookup(f.Raw)
		if t < 0 {
			continue
		}
		switch {
		case g.Kind[t] == kindPrimArray && g.ClassOf[t] == 8: // byte[]
			if s.body == "" {
				s.body = f.Name
			}
		case g.M.className(g.ClassOf[t]) == envelopeClass:
			s.envelope = f.Name
		case strings.Contains(g.M.className(g.ClassOf[t]), "BasicProperties"):
			s.props = f.Name
		case g.M.className(g.ClassOf[t]) == "java.lang.String":
			// prefer an explicitly named tag field over any other string
			if s.tag == "" || strings.Contains(strings.ToLower(f.Name), "tag") {
				s.tag = f.Name
			}
		}
	}
	s.ok = s.body != "" && s.envelope != ""
	cache[cid] = s
	return s
}

func (g *Graph) readDelivery(node, env int32, s *deliveryShape) delivery {
	d := delivery{node: node, holder: g.Name(node), bodyNode: -1}

	for _, f := range g.Fields(node) {
		switch f.Name {
		case s.body:
			if b := g.Lookup(f.Raw); b >= 0 {
				d.bodyNode = b
				d.bodyLen = int64(g.ArrayLen(b))
			}
		case s.tag:
			if v, ok := g.StringOf(g.Lookup(f.Raw)); ok {
				d.tag = v
			}
		case s.props:
			d.stamp = g.propsTimestamp(g.Lookup(f.Raw))
		}
	}
	// Envelope field names carry a leading underscore in the client library.
	for _, f := range g.Fields(env) {
		switch f.Name {
		case "_exchange", "exchange":
			if v, ok := g.StringOf(g.Lookup(f.Raw)); ok {
				d.exchange = v
			}
		case "_routingKey", "routingKey":
			if v, ok := g.StringOf(g.Lookup(f.Raw)); ok {
				d.routing = v
			}
		}
	}
	if d.tag == "" {
		d.tag = "<none>"
	}
	return d
}

// propsTimestamp reads BasicProperties.timestamp, the standard AMQP message
// timestamp, when the producer set one.
func (g *Graph) propsTimestamp(props int32) int64 {
	if props < 0 {
		return 0
	}
	f, ok := g.Field(props, "timestamp")
	if !ok || f.Raw == 0 {
		return 0
	}
	date := g.Lookup(f.Raw)
	if date < 0 {
		return 0
	}
	if ft, ok := g.Field(date, "fastTime"); ok {
		return int64(ft.Raw)
	}
	return 0
}

// reportBacklogAge prefers the AMQP message timestamp; when producers do not set
// one, an application-specific regex over the body can supply epoch millis.
func (g *Graph) reportBacklogAge(o *out, deliveries []delivery, stamps []int64, ageRegex string) {
	dumpTime := int64(g.M.Timestamp)
	source := "BasicProperties.timestamp"

	if len(stamps) == 0 && ageRegex != "" {
		re := mustCompile(ageRegex)
		if re.NumSubexp() < 1 {
			fatal("-age regex needs one capturing group for the epoch-millis value")
		}
		source = "body regex " + ageRegex
		for _, d := range deliveries {
			if d.bodyNode < 0 {
				continue
			}
			b, _ := g.PrimBytes(d.bodyNode, 512)
			m := re.FindSubmatch(b)
			if len(m) < 2 {
				continue
			}
			var ms int64
			if _, err := fmt.Sscan(string(m[1]), &ms); err == nil && ms > 1e12 {
				stamps = append(stamps, ms)
			}
		}
	}

	o.heading(3, "Backlog age")
	if len(stamps) == 0 {
		hint := "Producers did not set BasicProperties.timestamp."
		if ageRegex == "" {
			hint += " If your payloads embed a millisecond timestamp, pass `-age` with a regex whose first capture group is that value, e.g. `-age '\"ts\":(\\d{13})'`."
		}
		o.para("_no timestamps available — " + hint + "_")
		return
	}

	ages := make([]int64, 0, len(stamps))
	for _, s := range stamps {
		if a := dumpTime - s; a >= 0 {
			ages = append(ages, a)
		}
	}
	if len(ages) == 0 {
		o.para("_timestamps found but all postdate the dump — ignoring_")
		return
	}
	sort.Slice(ages, func(i, j int) bool { return ages[i] < ages[j] })

	o.para(fmt.Sprintf("Age at dump time, from %s (%d of %d deliveries carried one).",
		source, len(ages), len(deliveries)))
	o.table([]string{"p1", "p25", "p50", "p75", "p99", "oldest"}, "rrrrrr", [][]string{{
		durLabel(pct(ages, .01)), durLabel(pct(ages, .25)), durLabel(pct(ages, .50)),
		durLabel(pct(ages, .75)), durLabel(pct(ages, .99)), durLabel(ages[len(ages)-1]),
	}})

	buckets := []int64{60e3, 5 * 60e3, 15 * 60e3, 30 * 60e3, 60 * 60e3, 2 * 3600e3, 3 * 3600e3, 1 << 62}
	var rows [][]string
	prev := int64(0)
	for _, b := range buckets {
		n := 0
		for _, a := range ages {
			if a > prev && a <= b {
				n++
			}
		}
		label := fmt.Sprintf("%s – %s", durLabel(prev), durLabel(b))
		if b == 1<<62 {
			label = "older than " + durLabel(prev)
		}
		rows = append(rows, []string{label, fmt.Sprint(n)})
		prev = b
	}
	o.table([]string{"Age bucket", "Messages"}, "lr", rows)

	// A backlog with nothing recent means intake stopped, not that consumers slowed.
	newest := ages[0]
	if newest > 5*60e3 {
		o.para(fmt.Sprintf("**Nothing newer than %s.** The backlog stopped growing before the dump — intake had already halted, which points at the JVM stalling (GC death spiral) rather than at consumers merely running behind.", durLabel(newest)))
	}
}

// reportConsumers surfaces the Spring AMQP settings that decide whether the
// broker throttles delivery at all.
func (g *Graph) reportConsumers(o *out) {
	type row struct {
		queues, ack string
		prefetch    int32
		buffered    int32
	}
	var rows []row
	for i := 0; i < g.N(); i++ {
		if g.Kind[i] != kindInstance ||
			!strings.HasSuffix(g.M.className(g.ClassOf[i]), "BlockingQueueConsumer") {
			continue
		}
		r := row{}
		for _, f := range g.Fields(int32(i)) {
			switch f.Name {
			case "queues":
				var names []string
				if a := g.Lookup(f.Raw); a >= 0 {
					for _, e := range g.ArrayElems(a, 16) {
						if s, ok := g.StringOf(g.Lookup(e)); ok {
							names = append(names, s)
						}
					}
				}
				r.queues = strings.Join(names, ",")
			case "prefetchCount":
				r.prefetch = int32(f.Raw)
			case "acknowledgeMode":
				r.ack = g.RenderRef(f.Raw)
			case "queue":
				if q := g.Lookup(f.Raw); q >= 0 {
					if c, ok := g.Field(q, "count"); ok {
						if ai := g.Lookup(c.Raw); ai >= 0 {
							if v, ok := g.Field(ai, "value"); ok {
								r.buffered = int32(v.Raw)
							}
						}
					}
				}
			}
		}
		rows = append(rows, r)
	}
	if len(rows) == 0 {
		return
	}

	// Collapse the consumers of a queue into one row — they share a config.
	type key struct {
		queues, ack string
		prefetch    int32
	}
	grouped := map[key]*struct{ n, buffered int32 }{}
	var order []key
	for _, r := range rows {
		k := key{r.queues, r.ack, r.prefetch}
		v := grouped[k]
		if v == nil {
			v = &struct{ n, buffered int32 }{}
			grouped[k] = v
			order = append(order, k)
		}
		v.n++
		v.buffered += r.buffered
	}
	sort.Slice(order, func(i, j int) bool { return grouped[order[i]].n > grouped[order[j]].n })

	o.heading(3, "Spring AMQP consumers")
	var table [][]string
	noAck := false
	for _, k := range order {
		v := grouped[k]
		ack := k.ack
		if ack == "NONE" {
			ack = "NONE  ⚠"
			noAck = true
		}
		table = append(table, []string{
			truncate(k.queues, 52), fmt.Sprint(v.n), fmt.Sprint(k.prefetch), ack, fmt.Sprint(v.buffered),
		})
	}
	o.table([]string{"Queue(s)", "Consumers", "prefetch", "ackMode", "Buffered in Spring"}, "lrrlr", table)
	if noAck {
		o.para("**⚠ `ackMode=NONE` (no-ack): RabbitMQ does not apply QoS to no-ack consumers, so the configured prefetch is ignored and the broker pushes without limit.** The client's ConsumerWorkService queue is unbounded, so the backlog accumulates in the JVM heap until it is exhausted. Switching to `AUTO` restores flow control.")
	}
}

func (g *Graph) reportSamples(o *out, deliveries []delivery, samples int) {
	if samples <= 0 {
		return
	}
	o.heading(3, "Sample message bodies")
	// Spread the samples across the backlog rather than taking the first few.
	step := len(deliveries) / samples
	if step < 1 {
		step = 1
	}
	var b strings.Builder
	shown := 0
	for i := 0; i < len(deliveries) && shown < samples; i += step {
		d := deliveries[i]
		if d.bodyNode < 0 {
			continue
		}
		raw, _ := g.PrimBytes(d.bodyNode, 700)
		fmt.Fprintf(&b, "\n[%d bytes] %s / %s\n%s\n", d.bodyLen, d.exchange, d.routing, truncate(string(raw), 600))
		shown++
	}
	o.code(strings.TrimLeft(b.String(), "\n"))
}

func tallyRows(m map[string]*tally, top int) [][]string {
	var rows [][]string
	for i, r := range sortTally(m, true) {
		if i >= top {
			rows = append(rows, []string{fmt.Sprintf("… %d more", len(m)-top), "", ""})
			break
		}
		rows = append(rows, []string{truncate(r.Key, 66), fmt.Sprintf("%.2f", mb(r.Bytes)), fmt.Sprint(r.Count)})
	}
	return rows
}

func pct(sorted []int64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	return sorted[int(float64(len(sorted)-1)*p)]
}

func bytesLabel(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

func durLabel(ms int64) string {
	if ms >= 1<<61 {
		return "∞"
	}
	d := time.Duration(ms) * time.Millisecond
	switch {
	case d >= time.Hour:
		return fmt.Sprintf("%.1f h", d.Hours())
	case d >= time.Minute:
		return fmt.Sprintf("%.0f min", d.Minutes())
	}
	return fmt.Sprintf("%.0f s", d.Seconds())
}
