package asm

import (
	"fmt"
	"strings"
)

// srcLine is a line of source with its comment removed and split into words.
type srcLine struct {
	file   string
	line   int
	fields []string
}

// node is a statement of a script body; a repeat has the statements it repeats.
type node struct {
	srcLine
	kids []*node
}

func tokenize(file, text string) []srcLine {
	var out []srcLine
	for ln, raw := range strings.Split(text, "\n") {
		if i := strings.IndexByte(raw, ';'); i >= 0 {
			raw = raw[:i]
		}
		if f := strings.Fields(raw); len(f) > 0 {
			out = append(out, srcLine{file, ln + 1, f})
		}
	}
	return out
}

func isDeclaration(w string) bool {
	return w == "script" || w == "data" || w == "const" || w == "sound" || w == "pose" || w == "template"
}

// buildItems reads the sources into scripts and data, expanding the friendly statements.
// Named values (const, sound, pose) are shared by all files.
func buildItems(names []string, sources map[string]string) ([]item, error) {
	ctx := &lowerCtx{consts: map[string]int{}, poses: map[string]pose{}, vars: map[string]int{}}
	lines := map[string][]srcLine{}
	// Pass 1: the declarations of named values.
	for _, file := range names {
		lines[file] = tokenize(file, sources[file])
		ctx.file = file
		for _, l := range lines[file] {
			switch l.fields[0] {
			case "const", "sound":
				f := l.fields
				if len(f) < 4 || f[2] != "=" {
					return nil, fmt.Errorf("%s:%d: %s NAME = value", file, l.line, f[0])
				}
				expr := strings.Join(f[3:], "")
				v, ok := ctx.number(expr)
				if !ok {
					return nil, fmt.Errorf("%s:%d: %q is not a number or a named value", file, l.line, expr)
				}
				if _, dup := ctx.consts[f[1]]; dup {
					return nil, fmt.Errorf("%s:%d: %q is defined twice", file, l.line, f[1])
				}
				ctx.consts[f[1]] = v
			case "pose":
				if len(l.fields) < 2 {
					return nil, fmt.Errorf("%s:%d: pose NAME legs=A body=B", file, l.line)
				}
				_, kw := keywords(l.fields[2:])
				if kw["legs"] == "" || kw["body"] == "" {
					return nil, fmt.Errorf("%s:%d: pose NAME legs=A body=B", file, l.line)
				}
				ctx.poses[l.fields[1]] = pose{kw["legs"], kw["body"], kw["offset"]}
			}
		}
	}
	// Pass 2: scripts and data.
	templates := map[string]template{}
	var items []item
	for _, file := range names {
		ctx.file = file
		ls := lines[file]
		for i := 0; i < len(ls); {
			l := ls[i]
			switch l.fields[0] {
			case "const", "sound", "pose":
				i++
			case "template":
				if len(l.fields) < 2 {
					return nil, fmt.Errorf("%s:%d: template NAME [parameters]", file, l.line)
				}
				i++
				start := i
				for i < len(ls) && !isDeclaration(ls[i].fields[0]) {
					i++
				}
				templates[l.fields[1]] = template{params: l.fields[2:], body: ls[start:i], line: l.line}
			case "data":
				if len(l.fields) < 3 {
					return nil, fmt.Errorf("%s:%d: data needs a name and bytes", file, l.line)
				}
				var b []string
				for _, t := range l.fields[2:] {
					b = append(b, ctx.val(t))
				}
				items = append(items, item{d: &dataDef{name: l.fields[1], file: file, line: l.line, bytes: b}})
				i++
			case "script":
				if len(l.fields) < 2 {
					return nil, fmt.Errorf("%s:%d: script needs a name", file, l.line)
				}
				def := &scriptDef{name: l.fields[1], file: file}
				i++
				start := i
				for i < len(ls) && !isDeclaration(ls[i].fields[0]) {
					i++
				}
				text := ls[start:i]
				if len(l.fields) > 2 { // script NAME from TEMPLATE param=value ...
					var err error
					if text, err = instantiate(templates, l); err != nil {
						return nil, err
					}
				} else if len(text) == 0 {
					return nil, fmt.Errorf("%s:%d: script %s is empty", file, l.line, l.fields[1])
				}
				body, err := parseBlock(text)
				if err != nil {
					return nil, err
				}
				if err := ctx.expand(body, def); err != nil {
					return nil, err
				}
				items = append(items, item{s: def})
			default:
				return nil, fmt.Errorf("%s:%d: %q outside a script", file, l.line, l.fields[0])
			}
		}
	}
	return items, nil
}

// parseBlock turns lines into statements, collecting what is between "repeat N times" and end.
// (The engine has an instruction of its own called repeat, with two operands; it is not a block.)
func parseBlock(ls []srcLine) ([]*node, error) {
	var stack [][]*node
	var heads []*node
	cur := []*node{}
	for _, l := range ls {
		switch {
		case l.fields[0] == "repeat" && len(l.fields) >= 3 && l.fields[2] == "times":
			n := &node{srcLine: l}
			cur = append(cur, n)
			stack = append(stack, cur)
			heads = append(heads, n)
			cur = []*node{}
		case l.fields[0] == "end":
			if len(stack) == 0 {
				return nil, fmt.Errorf("%s:%d: end without repeat", l.file, l.line)
			}
			heads[len(heads)-1].kids = cur
			cur = stack[len(stack)-1]
			stack, heads = stack[:len(stack)-1], heads[:len(heads)-1]
		default:
			cur = append(cur, &node{srcLine: l})
		}
	}
	if len(stack) > 0 {
		h := heads[len(heads)-1]
		return nil, fmt.Errorf("%s:%d: repeat is never closed with end", h.file, h.line)
	}
	return cur, nil
}

// expand lowers statements into the engine's instructions, unrolling repeat blocks.
func (c *lowerCtx) expand(body []*node, def *scriptDef) error {
	for _, n := range body {
		f := n.fields
		if f[0] == "repeat" && len(f) >= 3 && f[2] == "times" {
			count, ok := c.number(f[1])
			if !ok || count < 0 || count > 1000 {
				return c.errf(n.line, "repeat %s times: need a number from 0 to 1000", f[1])
			}
			name := ""
			if len(f) == 5 && f[3] == "as" {
				name = f[4]
			} else if len(f) != 3 {
				return c.errf(n.line, "repeat N times [as NAME]")
			}
			saved, had := c.vars[name]
			for i := 0; i < count; i++ {
				if name != "" {
					c.vars[name] = i
				}
				if err := c.expand(n.kids, def); err != nil {
					return err
				}
			}
			if name != "" {
				if had {
					c.vars[name] = saved
				} else {
					delete(c.vars, name)
				}
			}
			continue
		}
		if f[0] == "mark" {
			def.stmts = append(def.stmts, stmt{n.line, f})
			continue
		}
		out, ok, err := c.lowerStmt(n.line, f)
		if err != nil {
			return err
		}
		if ok {
			def.stmts = append(def.stmts, out...)
			continue
		}
		// An instruction in the engine's own words, with named values filled in.
		low := []string{f[0]}
		for _, t := range f[1:] {
			low = append(low, c.val(t))
		}
		def.stmts = append(def.stmts, stmt{n.line, low})
	}
	return nil
}

// template is a script written once and used with different values, for the scripts that
// differ only in a sound or in which script comes next:
//
//	template idle fire
//	    controls facing right right=walk fire=fire default=self
//
//	script idle_pistol from idle fire=fire_pistol
//
// A parameter is replaced wherever it stands as a word (or after an = sign). self is the name of
// the script made from the template.
type template struct {
	params []string
	body   []srcLine
	line   int
}

func instantiate(templates map[string]template, l srcLine) ([]srcLine, error) {
	f := l.fields
	if len(f) < 4 || f[2] != "from" {
		return nil, fmt.Errorf("%s:%d: script NAME from TEMPLATE param=value ...", l.file, l.line)
	}
	t, ok := templates[f[3]]
	if !ok {
		return nil, fmt.Errorf("%s:%d: no template named %q", l.file, l.line, f[3])
	}
	values := map[string]string{"self": f[1]}
	_, kw := keywords(f[4:])
	for _, p := range t.params {
		v, set := kw[p]
		if !set {
			return nil, fmt.Errorf("%s:%d: template %s needs %s=...", l.file, l.line, f[3], p)
		}
		values[p] = v
		delete(kw, p)
	}
	for k := range kw {
		return nil, fmt.Errorf("%s:%d: template %s has no parameter %q", l.file, l.line, f[3], k)
	}
	out := make([]srcLine, len(t.body))
	for i, bl := range t.body {
		nf := make([]string, len(bl.fields))
		for j, tok := range bl.fields {
			nf[j] = replaceNames(tok, values)
		}
		out[i] = srcLine{bl.file, bl.line, nf}
	}
	return out, nil
}

// replaceNames replaces the names in a word (also inside "a+b" and after "key=") by their values.
func replaceNames(tok string, values map[string]string) string {
	if k, v, ok := strings.Cut(tok, "="); ok && k != "" { // the key of key=value is a word of the language
		return k + "=" + replaceNames(v, values)
	}
	var out strings.Builder
	isName := func(r byte) bool {
		return r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
	}
	for i := 0; i < len(tok); {
		if !isName(tok[i]) {
			out.WriteByte(tok[i])
			i++
			continue
		}
		j := i
		for j < len(tok) && isName(tok[j]) {
			j++
		}
		word := tok[i:j]
		if v, ok := values[word]; ok {
			word = v
		}
		out.WriteString(word)
		i = j
	}
	return out.String()
}
