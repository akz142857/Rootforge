// graph.go — object index and reference graph.
//
// Objects are enumerated once into flat arrays (id, class, size, file offset),
// indexed by a sorted-id binary search rather than a map so the memory cost
// stays ~28 bytes per object. References are then materialised as forward and
// reverse CSR adjacency, which is what the dominator and holder analyses need.
package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"sort"
)

// object kinds
const (
	kindClass = iota // java.lang.Class object (from CLASS_DUMP)
	kindInstance
	kindObjArray
	kindPrimArray
)

// Graph is the whole object graph plus a random-access handle to the dump.
type Graph struct {
	M *Meta

	IDs     []uint64 // object id, in file order
	ClassOf []uint64 // instance/array: class id. prim array: basic type code.
	Size    []int32  // shallow size in bytes
	Kind    []uint8
	Offset  []int64 // file offset of the sub-record body (just past the tag byte)

	sortedIDs []uint64 // IDs sorted ascending
	sortedIdx []int32  // sortedIDs[i] is IDs[sortedIdx[i]]

	FStart []int32 // forward CSR: refs of node i are FAdj[FStart[i]:FStart[i+1]]
	FAdj   []int32
	RStart []int32 // reverse CSR: referrers of node i
	RAdj   []int32

	Roots   []int32 // node indices that are GC roots
	RootSet map[int32]bool
	Unreach int64 // bytes not reachable from any GC root (filled by Dominators)
	file    *os.File
}

// N is the number of objects in the graph.
func (g *Graph) N() int { return len(g.IDs) }

// Lookup maps an object id to its node index, or -1.
func (g *Graph) Lookup(id uint64) int32 {
	i := sort.Search(len(g.sortedIDs), func(i int) bool { return g.sortedIDs[i] >= id })
	if i < len(g.sortedIDs) && g.sortedIDs[i] == id {
		return g.sortedIdx[i]
	}
	return -1
}

// Name renders a node's type the way a heap analyzer would show it.
func (g *Graph) Name(i int32) string {
	switch g.Kind[i] {
	case kindPrimArray:
		return typeName[g.ClassOf[i]] + "[]"
	case kindClass:
		return "class " + g.M.className(g.ClassOf[i])
	default:
		return g.M.className(g.ClassOf[i])
	}
}

// Label renders a node as "type @0xaddr".
func (g *Graph) Label(i int32) string {
	return fmt.Sprintf("%s @0x%x", g.Name(i), g.IDs[i])
}

// BuildGraph runs the object-enumeration and edge passes.
func BuildGraph(m *Meta, verbose bool, withEdges bool) *Graph {
	g := &Graph{M: m}

	// ---- pass A: enumerate objects ----
	logf(verbose, "indexing objects")
	d, total := openDump(m.Path)
	for d.pos < total {
		tag := d.u1()
		d.u4()
		length := int64(d.u4())
		if tag != tagHeapDump && tag != tagHeapSeg {
			d.skip(length)
			continue
		}
		end := d.pos + length
		for d.pos < end {
			sub := d.u1()
			if _, ok := skipRoot(d, sub); ok {
				continue
			}
			off := d.pos
			switch sub {
			case subClassDump:
				ci := readClassDump(d, m)
				g.add(ci.id, ci.id, 0, kindClass, off)
			case subInstance:
				oid := d.id()
				d.u4()
				cid := d.id()
				n := int64(d.u4())
				d.skip(n)
				g.add(oid, cid, int32(16+n), kindInstance, off)
			case subObjArray:
				oid := d.id()
				d.u4()
				n := int64(d.u4())
				acid := d.id()
				d.skip(n * 8)
				g.add(oid, acid, int32(16+n*8), kindObjArray, off)
			case subPrimArray:
				oid := d.id()
				d.u4()
				n := int64(d.u4())
				t := d.u1()
				d.skip(n * int64(typeSize[t]))
				g.add(oid, uint64(t), int32(16+n*int64(typeSize[t])), kindPrimArray, off)
			default:
				panic(fmt.Sprintf("hprof: unexpected sub-record 0x%02x", sub))
			}
		}
	}
	m.buildLayouts()

	// sorted id index for O(log n) lookups without a 400MB map
	n := g.N()
	g.sortedIdx = make([]int32, n)
	for i := range g.sortedIdx {
		g.sortedIdx[i] = int32(i)
	}
	sort.Slice(g.sortedIdx, func(a, b int) bool { return g.IDs[g.sortedIdx[a]] < g.IDs[g.sortedIdx[b]] })
	g.sortedIDs = make([]uint64, n)
	for i, p := range g.sortedIdx {
		g.sortedIDs[i] = g.IDs[p]
	}
	logf(verbose, "  %d objects", n)

	f, err := os.Open(m.Path)
	if err != nil {
		fatal("reopen %s: %v", m.Path, err)
	}
	g.file = f

	if !withEdges {
		return g
	}

	// ---- pass B: edges ----
	logf(verbose, "reading references")
	src := make([]int32, 0, n)
	dst := make([]int32, 0, n)
	emit := func(from int32, to uint64) {
		if to == 0 {
			return
		}
		if j := g.Lookup(to); j >= 0 {
			src = append(src, from)
			dst = append(dst, j)
		}
	}
	d2, total2 := openDump(m.Path)
	for d2.pos < total2 {
		tag := d2.u1()
		d2.u4()
		length := int64(d2.u4())
		if tag != tagHeapDump && tag != tagHeapSeg {
			d2.skip(length)
			continue
		}
		end := d2.pos + length
		for d2.pos < end {
			sub := d2.u1()
			if _, ok := skipRoot(d2, sub); ok {
				continue
			}
			switch sub {
			case subClassDump:
				ci := readClassDump(d2, m)
				me := g.Lookup(ci.id)
				for _, s := range ci.statics {
					emit(me, s)
				}
			case subInstance:
				oid := d2.id()
				d2.u4()
				cid := d2.id()
				sz := int(d2.u4())
				body := d2.take(sz)
				me := g.Lookup(oid)
				off := 0
				ci := m.Classes[cid]
				if ci != nil {
					for _, f := range ci.layout {
						w := typeSize[f.typ]
						if off+w > sz {
							break
						}
						if f.typ == 2 {
							emit(me, binary.BigEndian.Uint64(body[off:]))
						}
						off += w
					}
				}
			case subObjArray:
				oid := d2.id()
				d2.u4()
				count := int(d2.u4())
				d2.id()
				body := d2.take(count * 8)
				me := g.Lookup(oid)
				for i := 0; i < count; i++ {
					emit(me, binary.BigEndian.Uint64(body[i*8:]))
				}
			case subPrimArray:
				d2.id()
				d2.u4()
				cnt := int64(d2.u4())
				t := d2.u1()
				d2.skip(cnt * int64(typeSize[t]))
			}
		}
	}
	logf(verbose, "  %d references", len(src))

	g.FStart, g.FAdj = buildCSR(n, src, dst)
	g.RStart, g.RAdj = buildCSR(n, dst, src)

	g.RootSet = make(map[int32]bool, len(m.RootIDs))
	for _, rid := range m.RootIDs {
		if j := g.Lookup(rid); j >= 0 && !g.RootSet[j] {
			g.RootSet[j] = true
			g.Roots = append(g.Roots, j)
		}
	}
	sort.Slice(g.Roots, func(a, b int) bool { return g.Roots[a] < g.Roots[b] })
	return g
}

func (g *Graph) add(id, cls uint64, size int32, kind uint8, off int64) {
	g.IDs = append(g.IDs, id)
	g.ClassOf = append(g.ClassOf, cls)
	g.Size = append(g.Size, size)
	g.Kind = append(g.Kind, kind)
	g.Offset = append(g.Offset, off)
}

func buildCSR(n int, from, to []int32) ([]int32, []int32) {
	start := make([]int32, n+1)
	for _, f := range from {
		start[f+1]++
	}
	for i := 0; i < n; i++ {
		start[i+1] += start[i]
	}
	adj := make([]int32, len(to))
	fill := make([]int32, n)
	for k := range from {
		f := from[k]
		adj[start[f]+fill[f]] = to[k]
		fill[f]++
	}
	return start, adj
}

// Refs returns the nodes referenced by i.
func (g *Graph) Refs(i int32) []int32 { return g.FAdj[g.FStart[i]:g.FStart[i+1]] }

// Referrers returns the nodes referencing i.
func (g *Graph) Referrers(i int32) []int32 { return g.RAdj[g.RStart[i]:g.RStart[i+1]] }

// readAt pulls raw bytes back out of the dump for random-access inspection.
func (g *Graph) readAt(off int64, n int) []byte {
	b := make([]byte, n)
	if _, err := g.file.ReadAt(b, off); err != nil {
		return nil
	}
	return b
}
