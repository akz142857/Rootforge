// inspect.go — reading object contents back out of the dump.
//
// The graph pass keeps only ids, sizes and offsets, so anything that needs the
// actual field values re-reads that one object from the file. That keeps peak
// memory proportional to the object count rather than to the heap size, and
// makes field-level inspection cheap enough to use interactively.
package main

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf16"
)

// FieldValue is one instance field, with its raw slot contents.
type FieldValue struct {
	Name string
	Type byte
	Raw  uint64 // object fields hold the referenced id
}

// Fields reads the instance fields of a node. Returns nil for arrays.
func (g *Graph) Fields(i int32) []FieldValue {
	if g.Kind[i] != kindInstance {
		return nil
	}
	head := g.readAt(g.Offset[i], 8+4+8+4)
	if head == nil {
		return nil
	}
	cid := binary.BigEndian.Uint64(head[12:])
	size := int(binary.BigEndian.Uint32(head[20:]))
	body := g.readAt(g.Offset[i]+24, size)
	if body == nil {
		return nil
	}
	ci := g.M.Classes[cid]
	if ci == nil {
		return nil
	}
	out := make([]FieldValue, 0, len(ci.layout))
	off := 0
	for _, f := range ci.layout {
		w := typeSize[f.typ]
		if off+w > size {
			break
		}
		var raw uint64
		switch f.typ {
		case 2:
			raw = binary.BigEndian.Uint64(body[off:])
		case 4, 8:
			raw = uint64(body[off])
		case 5, 9:
			raw = uint64(binary.BigEndian.Uint16(body[off:]))
		case 6, 10:
			raw = uint64(binary.BigEndian.Uint32(body[off:]))
		case 7, 11:
			raw = binary.BigEndian.Uint64(body[off:])
		}
		out = append(out, FieldValue{f.Name(), f.typ, raw})
		off += w
	}
	return out
}

func (f fieldDesc) Name() string { return f.name }

// Field returns one named field of a node.
func (g *Graph) Field(i int32, name string) (FieldValue, bool) {
	for _, f := range g.Fields(i) {
		if f.Name == name {
			return f, true
		}
	}
	return FieldValue{}, false
}

// ArrayLen returns the element count of an array node.
func (g *Graph) ArrayLen(i int32) int {
	switch g.Kind[i] {
	case kindObjArray, kindPrimArray:
		h := g.readAt(g.Offset[i]+12, 4)
		if h == nil {
			return 0
		}
		return int(binary.BigEndian.Uint32(h))
	}
	return 0
}

// ArrayElems returns up to max element ids of an object array.
func (g *Graph) ArrayElems(i int32, max int) []uint64 {
	if g.Kind[i] != kindObjArray {
		return nil
	}
	n := g.ArrayLen(i)
	if n > max {
		n = max
	}
	b := g.readAt(g.Offset[i]+24, n*8)
	if b == nil {
		return nil
	}
	out := make([]uint64, n)
	for k := 0; k < n; k++ {
		out[k] = binary.BigEndian.Uint64(b[k*8:])
	}
	return out
}

// PrimBytes returns up to max raw bytes of a primitive array.
func (g *Graph) PrimBytes(i int32, max int) ([]byte, byte) {
	if g.Kind[i] != kindPrimArray {
		return nil, 0
	}
	h := g.readAt(g.Offset[i]+12, 5)
	if h == nil {
		return nil, 0
	}
	n := int(binary.BigEndian.Uint32(h))
	t := h[4]
	total := n * typeSize[t]
	if total > max {
		total = max
	}
	return g.readAt(g.Offset[i]+17, total), t
}

// StringOf decodes a java.lang.String node. Handles both the byte[]+coder
// layout (JDK 9+ compact strings) and the older char[] layout.
func (g *Graph) StringOf(i int32) (string, bool) {
	if i < 0 || g.Kind[i] != kindInstance || g.M.className(g.ClassOf[i]) != "java.lang.String" {
		return "", false
	}
	var valID uint64
	coder := uint64(0)
	hasCoder := false
	for _, f := range g.Fields(i) {
		switch f.Name {
		case "value":
			valID = f.Raw
		case "coder":
			coder, hasCoder = f.Raw, true
		}
	}
	v := g.Lookup(valID)
	if v < 0 {
		return "", false
	}
	b, t := g.PrimBytes(v, 1<<20)
	if b == nil {
		return "", false
	}
	if t == 5 { // char[]: UTF-16BE
		return decodeUTF16(b, true), true
	}
	if hasCoder && coder == 1 { // UTF-16LE
		return decodeUTF16(b, false), true
	}
	return string(b), true
}

func decodeUTF16(b []byte, bigEndian bool) string {
	u := make([]uint16, len(b)/2)
	for i := range u {
		if bigEndian {
			u[i] = binary.BigEndian.Uint16(b[i*2:])
		} else {
			u[i] = binary.LittleEndian.Uint16(b[i*2:])
		}
	}
	return string(utf16.Decode(u))
}

// isEnum reports whether a class descends from java.lang.Enum.
func (g *Graph) isEnum(cid uint64) bool {
	for hops := 0; hops < 16 && cid != 0; hops++ {
		ci := g.M.Classes[cid]
		if ci == nil {
			return false
		}
		if ci.name == "java.lang.Enum" {
			return true
		}
		cid = ci.super
	}
	return false
}

var boxedField = map[string]bool{
	"java.lang.Integer": true, "java.lang.Long": true, "java.lang.Short": true,
	"java.lang.Byte": true, "java.lang.Character": true, "java.lang.Boolean": true,
	"java.lang.Double": true, "java.lang.Float": true,
	"java.util.concurrent.atomic.AtomicInteger": true,
	"java.util.concurrent.atomic.AtomicLong":    true,
	"java.util.concurrent.atomic.AtomicBoolean": true,
}

// sizeFields are the conventional element-count fields of common containers,
// shown inline so a collection reference is informative on its own.
var sizeFields = map[string]string{
	"java.util.ArrayList": "size", "java.util.LinkedList": "size",
	"java.util.HashMap": "size", "java.util.LinkedHashMap": "size",
	"java.util.TreeMap": "size", "java.util.HashSet": "size",
	"java.util.concurrent.LinkedBlockingQueue":  "count",
	"java.util.concurrent.ArrayBlockingQueue":   "count",
	"java.util.concurrent.CopyOnWriteArrayList": "",
}

// Render renders a field value for display. Object references are resolved one
// level deep — strings, enums, boxed primitives and container sizes — which is
// almost always what you want and never explodes the output.
func (g *Graph) Render(f FieldValue) string {
	switch f.Type {
	case 4:
		if f.Raw != 0 {
			return "true"
		}
		return "false"
	case 5:
		return fmt.Sprintf("'%c'", rune(f.Raw))
	case 8:
		return fmt.Sprint(int8(f.Raw))
	case 9:
		return fmt.Sprint(int16(f.Raw))
	case 10:
		return fmt.Sprint(int32(f.Raw))
	case 11:
		return fmt.Sprint(int64(f.Raw))
	case 6:
		return fmt.Sprintf("%g", float32frombits(uint32(f.Raw)))
	case 7:
		return fmt.Sprintf("%g", float64frombits(f.Raw))
	case 2:
		return g.RenderRef(f.Raw)
	}
	return fmt.Sprint(f.Raw)
}

// RenderRef renders an object reference one level deep.
func (g *Graph) RenderRef(id uint64) string {
	if id == 0 {
		return "null"
	}
	i := g.Lookup(id)
	if i < 0 {
		return fmt.Sprintf("0x%x (not in dump)", id)
	}
	name := g.Name(i)

	if s, ok := g.StringOf(i); ok {
		return fmt.Sprintf("%q", truncate(s, 160))
	}
	if g.Kind[i] == kindInstance && g.isEnum(g.ClassOf[i]) {
		if nf, ok := g.Field(i, "name"); ok {
			if s, ok := g.StringOf(g.Lookup(nf.Raw)); ok {
				return s
			}
		}
	}
	if boxedField[name] {
		if vf, ok := g.Field(i, "value"); ok {
			return fmt.Sprintf("%s(%s)", shortName(name), g.Render(vf))
		}
	}
	switch g.Kind[i] {
	case kindPrimArray:
		n := g.ArrayLen(i)
		b, t := g.PrimBytes(i, 64)
		if t == 8 || t == 5 {
			return fmt.Sprintf("%s[%d] %q", typeName[t], n, truncate(printable(b, t), 64))
		}
		return fmt.Sprintf("%s[%d]", typeName[t], n)
	case kindObjArray:
		n := g.ArrayLen(i)
		label := fmt.Sprintf("%s[%d]", strings.TrimSuffix(shortName(name), "[]"), n)
		// Short arrays are usually configuration — queue names, hostnames — so
		// show them inline rather than making the caller chase the address.
		if n > 0 && n <= 8 {
			elems := make([]string, 0, n)
			for _, e := range g.ArrayElems(i, n) {
				elems = append(elems, g.renderElem(e))
			}
			return fmt.Sprintf("%s{%s}", label, strings.Join(elems, ", "))
		}
		return fmt.Sprintf("%s @0x%x", label, id)
	}
	if sf, ok := sizeFields[name]; ok && sf != "" {
		if vf, ok := g.Field(i, sf); ok {
			sz := g.Render(vf)
			if vf.Type == 2 { // e.g. LinkedBlockingQueue.count is an AtomicInteger
				if ai := g.Lookup(vf.Raw); ai >= 0 {
					if inner, ok := g.Field(ai, "value"); ok {
						sz = g.Render(inner)
					}
				}
			}
			return fmt.Sprintf("%s(%s) @0x%x", shortName(name), sz, id)
		}
	}
	return fmt.Sprintf("%s @0x%x", shortName(name), id)
}

// renderElem renders an array element without recursing into further arrays.
func (g *Graph) renderElem(id uint64) string {
	if id == 0 {
		return "null"
	}
	i := g.Lookup(id)
	if i < 0 {
		return fmt.Sprintf("0x%x", id)
	}
	if s, ok := g.StringOf(i); ok {
		return fmt.Sprintf("%q", truncate(s, 80))
	}
	if g.Kind[i] == kindInstance && g.isEnum(g.ClassOf[i]) {
		if nf, ok := g.Field(i, "name"); ok {
			if s, ok := g.StringOf(g.Lookup(nf.Raw)); ok {
				return s
			}
		}
	}
	if boxedField[g.Name(i)] {
		if vf, ok := g.Field(i, "value"); ok {
			return g.Render(vf)
		}
	}
	return fmt.Sprintf("%s@0x%x", shortName(g.Name(i)), id)
}

// EdgeLabel names the reference from parent to child: a field name for an
// instance, an index for an array, "static" for a class object.
func (g *Graph) EdgeLabel(parent, child int32) string {
	switch g.Kind[parent] {
	case kindInstance:
		want := g.IDs[child]
		for _, f := range g.Fields(parent) {
			if f.Type == 2 && f.Raw == want {
				return "." + f.Name
			}
		}
	case kindObjArray:
		want := g.IDs[child]
		for k, e := range g.ArrayElems(parent, 1<<20) {
			if e == want {
				return fmt.Sprintf("[%d]", k)
			}
		}
	case kindClass:
		return " (static)"
	}
	return ""
}

func printable(b []byte, t byte) string {
	var sb strings.Builder
	if t == 5 {
		for i := 0; i+1 < len(b); i += 2 {
			c := rune(binary.BigEndian.Uint16(b[i:]))
			sb.WriteRune(printableRune(c))
		}
		return sb.String()
	}
	for _, c := range b {
		sb.WriteRune(printableRune(rune(c)))
	}
	return sb.String()
}

func printableRune(c rune) rune {
	if c >= 32 && c < 127 {
		return c
	}
	return '.'
}

func shortName(s string) string {
	if i := strings.LastIndexByte(s, '.'); i >= 0 {
		return s[i+1:]
	}
	return s
}
