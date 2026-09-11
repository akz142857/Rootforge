// Package main — hprofx: a self-contained HPROF (Java heap dump) analyzer.
//
// hprof.go holds the binary reader and the metadata pass: strings, class names,
// class dumps, instance field layouts, stack frames and stack traces.
package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
)

// ---- HPROF record tags (top level) ----
const (
	tagString     = 0x01
	tagLoadClass  = 0x02
	tagStackFrame = 0x04
	tagStackTrace = 0x05
	tagHeapDump   = 0x0C
	tagHeapSeg    = 0x1C
)

// ---- heap dump sub-record tags ----
const (
	subClassDump = 0x20
	subInstance  = 0x21
	subObjArray  = 0x22
	subPrimArray = 0x23
	subHeapInfo  = 0xFE
)

// basic type codes used by field descriptors and primitive arrays
var typeSize = [12]int{0, 0, 8, 0, 1, 2, 4, 8, 1, 2, 4, 8}
var typeName = [12]string{"", "", "object", "", "boolean", "char", "float", "double", "byte", "short", "int", "long"}

// reader is a forward-only buffered reader that tracks its byte offset so we can
// bound heap-dump segments and record per-object file offsets for random access.
type reader struct {
	r   *bufio.Reader
	pos int64
	buf [8]byte
}

func (d *reader) must(n int) []byte {
	b := d.buf[:n]
	if _, err := io.ReadFull(d.r, b); err != nil {
		panic(fmt.Sprintf("hprof: read %d bytes at offset %d: %v", n, d.pos, err))
	}
	d.pos += int64(n)
	return b
}

func (d *reader) u1() byte   { return d.must(1)[0] }
func (d *reader) u2() uint16 { return binary.BigEndian.Uint16(d.must(2)) }
func (d *reader) u4() uint32 { return binary.BigEndian.Uint32(d.must(4)) }
func (d *reader) i4() int32  { return int32(binary.BigEndian.Uint32(d.must(4))) }
func (d *reader) u8() uint64 { return binary.BigEndian.Uint64(d.must(8)) }
func (d *reader) id() uint64 { return d.u8() } // all dumps we target use 8-byte ids
func (d *reader) take(n int) []byte {
	b := make([]byte, n)
	if _, err := io.ReadFull(d.r, b); err != nil {
		panic(fmt.Sprintf("hprof: take %d bytes at offset %d: %v", n, d.pos, err))
	}
	d.pos += int64(n)
	return b
}

func (d *reader) skip(n int64) {
	for n > 0 {
		chunk := n
		if chunk > 1<<20 {
			chunk = 1 << 20
		}
		k, err := d.r.Discard(int(chunk))
		d.pos += int64(k)
		n -= int64(k)
		if err != nil {
			panic(fmt.Sprintf("hprof: skip at offset %d: %v", d.pos, err))
		}
	}
}

// readValue consumes one field value of the given basic type and returns it
// widened to uint64 (object fields yield the referenced id).
func (d *reader) readValue(t byte) uint64 {
	switch t {
	case 2:
		return d.id()
	case 4, 8:
		return uint64(d.u1())
	case 5, 9:
		return uint64(d.u2())
	case 6, 10:
		return uint64(d.u4())
	case 7, 11:
		return d.u8()
	}
	panic(fmt.Sprintf("hprof: unknown basic type %d at offset %d", t, d.pos))
}

type fieldDesc struct {
	name string
	typ  byte
}

type classInfo struct {
	id       uint64
	name     string
	super    uint64
	instSize int32
	fields   []fieldDesc // declared by this class only
	statics  []uint64    // object-typed static field values
	layout   []fieldDesc // this class then each superclass, in HPROF instance order
}

// Meta is everything learned from the metadata pass — cheap to compute and
// needed by every command.
type Meta struct {
	Path      string
	FileSize  int64
	Timestamp uint64
	IDSize    int

	Strings map[uint64]string
	Classes map[uint64]*classInfo
	BySer   map[uint32]string // class serial -> name, for stack frames

	Frames  map[uint64]string   // frame id -> rendered "Class.method (File:line)"
	Traces  map[uint32][]uint64 // stack serial -> frame ids
	Threads []ThreadRef

	RootIDs   []uint64
	RootKinds map[string]int64
}

// ThreadRef links a java.lang.Thread object to its stack trace.
type ThreadRef struct {
	ObjID       uint64
	Serial      uint32
	StackSerial uint32
}

// lastTimestamp carries the millisecond timestamp from the most recently
// opened dump header; dumpTimestamp reads it without a full metadata pass.
var lastTimestamp uint64

func dumpTimestamp(path string) uint64 {
	openDump(path)
	return lastTimestamp
}

func openDump(path string) (*reader, int64) {
	f, err := os.Open(path)
	if err != nil {
		fatal("cannot open %s: %v", path, err)
	}
	fi, err := f.Stat()
	if err != nil {
		fatal("cannot stat %s: %v", path, err)
	}
	d := &reader{r: bufio.NewReaderSize(f, 16<<20)}
	// header: NUL-terminated format string, then u4 id size, then u8 timestamp
	var hdr []byte
	for {
		c := d.u1()
		if c == 0 {
			break
		}
		hdr = append(hdr, c)
	}
	idSize := int(d.u4())
	if idSize != 8 {
		fatal("unsupported identifier size %d (only 64-bit dumps are supported)", idSize)
	}
	lastTimestamp = d.u8()
	if !strings.HasPrefix(string(hdr), "JAVA PROFILE") {
		fatal("not an HPROF dump (header %q)", string(hdr))
	}
	return d, fi.Size()
}

// skipRoot consumes a GC-root sub-record. Returns the referenced object id
// (0 when the record carries none) and whether the tag was in fact a root.
func skipRoot(d *reader, tag byte) (uint64, bool) {
	switch tag {
	case 0xFF: // unknown
		return d.id(), true
	case 0x01: // JNI global
		v := d.id()
		d.id()
		return v, true
	case 0x02, 0x03, 0x08: // JNI local, java frame, thread object
		v := d.id()
		d.u4()
		d.u4()
		return v, true
	case 0x04, 0x06: // native stack, thread block
		v := d.id()
		d.u4()
		return v, true
	case 0x05, 0x07: // sticky class, monitor used
		return d.id(), true
	case subHeapInfo: // Android heap partition marker
		d.u4()
		d.id()
		return 0, true
	}
	return 0, false
}

var rootTagName = map[byte]string{
	0xFF: "unknown", 0x01: "jni_global", 0x02: "jni_local", 0x03: "java_frame",
	0x04: "native_stack", 0x05: "sticky_class", 0x06: "thread_block",
	0x07: "monitor_used", 0x08: "thread_object",
}

// readClassDump parses a CLASS_DUMP sub-record.
//
// Class dumps are re-read on every pass over the file, so this updates the
// existing classInfo in place rather than replacing it — replacing it would
// discard the flattened field layout and silently drop every instance-field
// reference from the graph.
func readClassDump(d *reader, m *Meta) *classInfo {
	id := d.id()
	d.u4() // stack trace serial
	super := d.id()
	d.id() // loader
	d.id() // signers
	d.id() // protection domain
	d.id() // reserved
	d.id() // reserved
	instSize := int32(d.u4())

	cpCount := int(d.u2())
	for i := 0; i < cpCount; i++ {
		d.u2()
		d.readValue(d.u1())
	}
	staticCount := int(d.u2())
	var statics []uint64
	for i := 0; i < staticCount; i++ {
		d.id()
		t := d.u1()
		v := d.readValue(t)
		if t == 2 && v != 0 {
			statics = append(statics, v)
		}
	}
	fieldCount := int(d.u2())
	fields := make([]fieldDesc, fieldCount)
	for i := 0; i < fieldCount; i++ {
		nameID := d.id()
		fields[i] = fieldDesc{m.Strings[nameID], d.u1()}
	}

	ci := m.Classes[id]
	if ci == nil {
		ci = &classInfo{id: id}
		m.Classes[id] = ci
	}
	ci.super = super
	ci.instSize = instSize
	ci.statics = statics
	if ci.layout == nil {
		ci.fields = fields
	}
	return ci
}

// skipObjectBody consumes an instance/array sub-record without interpreting it.
func skipObjectBody(d *reader, sub byte) {
	switch sub {
	case subInstance:
		d.id()
		d.u4()
		d.id()
		d.skip(int64(d.u4()))
	case subObjArray:
		d.id()
		d.u4()
		n := int64(d.u4())
		d.id()
		d.skip(n * 8)
	case subPrimArray:
		d.id()
		d.u4()
		n := int64(d.u4())
		t := d.u1()
		d.skip(n * int64(typeSize[t]))
	default:
		panic(fmt.Sprintf("hprof: unexpected sub-record 0x%02x at offset %d", sub, d.pos))
	}
}

// normalizeClassName turns a JVM internal name into source form, decoding
// array descriptors so "[Ljava/util/HashMap$Node;" reads as
// "java.util.HashMap$Node[]" rather than as raw bytecode.
func normalizeClassName(s string) string {
	s = strings.ReplaceAll(s, "/", ".")
	if !strings.HasPrefix(s, "[") {
		return s
	}
	dims := 0
	for dims < len(s) && s[dims] == '[' {
		dims++
	}
	elem := s[dims:]
	switch {
	case elem == "":
		return s
	case elem[0] == 'L':
		elem = strings.TrimSuffix(elem[1:], ";")
	default:
		prim, ok := primDescriptor[elem[0]]
		if !ok {
			return s
		}
		elem = prim
	}
	return elem + strings.Repeat("[]", dims)
}

var primDescriptor = map[byte]string{
	'B': "byte", 'C': "char", 'D': "double", 'F': "float",
	'I': "int", 'J': "long", 'S': "short", 'Z': "boolean",
}

// LoadMeta runs the metadata pass: everything except object contents.
func LoadMeta(path string) *Meta {
	m := &Meta{
		Path:      path,
		IDSize:    8,
		Strings:   make(map[uint64]string, 1<<20),
		Classes:   make(map[uint64]*classInfo, 1<<16),
		BySer:     make(map[uint32]string, 1<<16),
		Frames:    make(map[uint64]string, 1<<16),
		Traces:    make(map[uint32][]uint64, 1<<12),
		RootKinds: make(map[string]int64),
	}
	d, total := openDump(path)
	m.FileSize = total

	for d.pos < total {
		tag := d.u1()
		d.u4() // time delta
		length := int64(d.u4())
		switch tag {
		case tagString:
			id := d.id()
			m.Strings[id] = string(d.take(int(length) - 8))
		case tagLoadClass:
			serial := d.u4()
			cid := d.id()
			d.u4()
			name := normalizeClassName(m.Strings[d.id()])
			m.BySer[serial] = name
			if ci := m.Classes[cid]; ci != nil {
				ci.name = name
			} else {
				m.Classes[cid] = &classInfo{id: cid, name: name}
			}
		case tagStackFrame:
			fid := d.id()
			method := m.Strings[d.id()]
			sig := m.Strings[d.id()]
			source := m.Strings[d.id()]
			classSer := d.u4()
			line := d.i4()
			m.Frames[fid] = fmt.Sprintf("%s.%s%s (%s:%s)",
				m.BySer[classSer], method, argsOf(sig), source, lineLabel(line))
		case tagStackTrace:
			serial := d.u4()
			d.u4() // thread serial
			n := int(d.u4())
			fs := make([]uint64, n)
			for i := range fs {
				fs[i] = d.id()
			}
			m.Traces[serial] = fs
		case tagHeapDump, tagHeapSeg:
			end := d.pos + length
			for d.pos < end {
				sub := d.u1()
				if sub == 0x08 { // thread object root also links a stack trace
					oid := d.id()
					serial := d.u4()
					stack := d.u4()
					m.Threads = append(m.Threads, ThreadRef{oid, serial, stack})
					m.RootKinds["thread_object"]++
					m.RootIDs = append(m.RootIDs, oid)
					continue
				}
				if rid, ok := skipRoot(d, sub); ok {
					m.RootKinds[rootTagName[sub]]++
					if rid != 0 {
						m.RootIDs = append(m.RootIDs, rid)
					}
					continue
				}
				if sub == subClassDump {
					readClassDump(d, m)
					continue
				}
				skipObjectBody(d, sub)
			}
		default:
			d.skip(length)
		}
	}
	m.buildLayouts()
	return m
}

// buildLayouts flattens each class's instance fields with its superclasses, in
// the order HPROF writes them (own fields first, then up the chain).
func (m *Meta) buildLayouts() {
	var build func(uint64) []fieldDesc
	seen := map[uint64]bool{}
	build = func(cid uint64) []fieldDesc {
		ci := m.Classes[cid]
		if ci == nil {
			return nil
		}
		if ci.layout != nil || seen[cid] {
			return ci.layout
		}
		seen[cid] = true
		out := append([]fieldDesc{}, ci.fields...)
		if ci.super != 0 {
			out = append(out, build(ci.super)...)
		}
		ci.layout = out
		return out
	}
	for cid := range m.Classes {
		build(cid)
	}
}

func (m *Meta) className(cid uint64) string {
	if ci := m.Classes[cid]; ci != nil && ci.name != "" {
		return ci.name
	}
	return fmt.Sprintf("<class 0x%x>", cid)
}

func argsOf(sig string) string {
	if i := strings.Index(sig, ")"); i >= 0 {
		return sig[:i+1]
	}
	return ""
}

func lineLabel(line int32) string {
	switch line {
	case -1:
		return "unknown"
	case -2:
		return "compiled"
	case -3:
		return "native"
	}
	return fmt.Sprint(line)
}
