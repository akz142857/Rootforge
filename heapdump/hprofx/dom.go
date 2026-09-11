// dom.go — dominator tree and retained sizes.
//
// Retained size is what makes a heap dump actionable: the bytes that would be
// freed if one object became unreachable. It is computed from the dominator
// tree of the object graph rooted at a virtual node joined to every GC root.
//
// This uses Lengauer-Tarjan with path compression rather than the simpler
// iterative (Cooper-Harvey-Kennedy) formulation: heap graphs are deeply cyclic
// and irreducible, and the iterative version can need far more rounds than a
// sane cap allows, silently yielding approximate numbers.
package main

// Dom holds the dominator tree and retained sizes for a graph.
type Dom struct {
	g *Graph

	Root     int32   // virtual super-root, index == g.N()
	Idom     []int32 // immediate dominator per node (-1 when unreachable)
	Retained []int64 // retained bytes per node (0 when unreachable)
	Vertex   []int32 // nodes in DFS preorder
	dfn      []int32 // node -> DFS number, -1 when unreachable

	parent   []int32
	semi     []int32
	label    []int32
	ancestor []int32
	bhead    []int32
	bnext    []int32
	scratch  []int32
}

// Reachable reports whether a node is reachable from any GC root.
func (d *Dom) Reachable(i int32) bool { return d.dfn[i] >= 0 }

// Dominators computes the dominator tree and retained sizes.
func Dominators(g *Graph, verbose bool) *Dom {
	n := g.N()
	root := int32(n)
	size := n + 1

	d := &Dom{
		g:        g,
		Root:     root,
		Idom:     make([]int32, size),
		Retained: make([]int64, size),
		dfn:      make([]int32, size),
		parent:   make([]int32, size),
		semi:     make([]int32, size),
		label:    make([]int32, size),
		ancestor: make([]int32, size),
		bhead:    make([]int32, size),
		bnext:    make([]int32, size),
	}
	for i := 0; i < size; i++ {
		d.dfn[i] = -1
		d.Idom[i] = -1
		d.parent[i] = -1
		d.ancestor[i] = -1
		d.bhead[i] = -1
		d.bnext[i] = -1
		d.label[i] = int32(i)
	}

	// successors: the virtual root fans out to every GC root
	succ := func(v int32) []int32 {
		if v == root {
			return g.Roots
		}
		return g.Refs(v)
	}

	logf(verbose, "walking object graph")
	d.Vertex = make([]int32, 0, size)
	type frame struct {
		v    int32
		next int32
	}
	stack := make([]frame, 0, 1<<16)
	d.dfn[root] = 0
	d.Vertex = append(d.Vertex, root)
	stack = append(stack, frame{root, 0})
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		kids := succ(top.v)
		if int(top.next) < len(kids) {
			w := kids[top.next]
			top.next++
			if d.dfn[w] < 0 {
				d.dfn[w] = int32(len(d.Vertex))
				d.parent[w] = top.v
				d.Vertex = append(d.Vertex, w)
				stack = append(stack, frame{w, 0})
			}
			continue
		}
		stack = stack[:len(stack)-1]
	}
	for i, v := range d.Vertex {
		d.semi[v] = int32(i)
	}
	logf(verbose, "  %d of %d objects reachable", len(d.Vertex)-1, n)

	// predecessors: GC roots additionally have the virtual root as a predecessor
	logf(verbose, "computing dominators")
	for i := len(d.Vertex) - 1; i >= 1; i-- {
		w := d.Vertex[i]
		for _, v := range g.Referrers(w) {
			if d.dfn[v] < 0 {
				continue // unreachable predecessor contributes nothing
			}
			if u := d.eval(v); d.semi[u] < d.semi[w] {
				d.semi[w] = d.semi[u]
			}
		}
		if g.RootSet[w] && d.semi[root] < d.semi[w] {
			d.semi[w] = d.semi[root]
		}

		b := d.Vertex[d.semi[w]]
		d.bnext[w] = d.bhead[b]
		d.bhead[b] = w

		p := d.parent[w]
		d.ancestor[w] = p // LINK

		for v := d.bhead[p]; v != -1; v = d.bnext[v] {
			if u := d.eval(v); d.semi[u] < d.semi[v] {
				d.Idom[v] = u
			} else {
				d.Idom[v] = p
			}
		}
		d.bhead[p] = -1
	}
	for i := 1; i < len(d.Vertex); i++ {
		w := d.Vertex[i]
		if d.Idom[w] != d.Vertex[d.semi[w]] {
			d.Idom[w] = d.Idom[d.Idom[w]]
		}
	}
	d.Idom[root] = root

	// retained size: fold each node into its dominator, deepest first
	for i := 0; i < n; i++ {
		if d.dfn[i] >= 0 {
			d.Retained[i] = int64(g.Size[i])
		} else {
			g.Unreach += int64(g.Size[i])
		}
	}
	for i := len(d.Vertex) - 1; i >= 1; i-- {
		w := d.Vertex[i]
		if p := d.Idom[w]; p >= 0 && p != w {
			d.Retained[p] += d.Retained[w]
		}
	}
	return d
}

func (d *Dom) eval(v int32) int32 {
	if d.ancestor[v] == -1 {
		return v
	}
	d.compress(v)
	return d.label[v]
}

// compress is the iterative form of Lengauer-Tarjan's COMPRESS; the recursive
// version overflows the stack on the long reference chains heap dumps contain.
func (d *Dom) compress(v int32) {
	path := d.scratch[:0]
	u := v
	for d.ancestor[d.ancestor[u]] != -1 {
		path = append(path, u)
		u = d.ancestor[u]
	}
	for i := len(path) - 1; i >= 0; i-- {
		x := path[i]
		a := d.ancestor[x]
		if d.semi[d.label[a]] < d.semi[d.label[x]] {
			d.label[x] = d.label[a]
		}
		d.ancestor[x] = d.ancestor[a]
	}
	d.scratch = path
}

// DomChain returns the dominator ancestors of a node, nearest first.
func (d *Dom) DomChain(i int32, max int) []int32 {
	var out []int32
	v := d.Idom[i]
	for len(out) < max && v >= 0 && v != d.Root {
		out = append(out, v)
		v = d.Idom[v]
	}
	return out
}
