package db

import (
	"strconv"
	"strings"
)

type bqNodeKind int

const (
	bqTerm bqNodeKind = iota
	bqAnd
	bqOr
	bqNot
)

type bqNode struct {
	kind     bqNodeKind
	text     string
	children []*bqNode
}

type bqParser struct {
	src []rune
	i   int
}

func (p *bqParser) skipSpace() {
	for p.i < len(p.src) && p.src[p.i] == ' ' {
		p.i++
	}
}

func (p *bqParser) peekOperator(word string) bool {
	if p.i+len(word) > len(p.src) {
		return false
	}
	if string(p.src[p.i:p.i+len(word)]) != word {
		return false
	}
	j := p.i + len(word)
	if j < len(p.src) && p.src[j] != ' ' && p.src[j] != '(' && p.src[j] != ')' {
		return false
	}
	return true
}

func (p *bqParser) parseQuery() (*bqNode, bool) {
	n, ok := p.parseOr()
	if !ok {
		return nil, false
	}
	p.skipSpace()
	if p.i < len(p.src) {
		return nil, false
	}
	return n, true
}

func (p *bqParser) parseOr() (*bqNode, bool) {
	left, ok := p.parseAnd()
	if !ok {
		return nil, false
	}
	for {
		p.skipSpace()
		if p.peekOperator("OR") {
			p.i += 2
		} else if p.i < len(p.src) && p.src[p.i] == '|' {
			p.i++
		} else {
			return left, true
		}
		right, ok := p.parseAnd()
		if !ok {
			return nil, false
		}
		left = &bqNode{kind: bqOr, children: []*bqNode{left, right}}
	}
}

func (p *bqParser) parseAnd() (*bqNode, bool) {
	var parts []*bqNode
	for {
		p.skipSpace()
		if p.i >= len(p.src) {
			break
		}
		c := p.src[p.i]
		if c == ')' || c == '|' {
			break
		}
		if p.peekOperator("OR") {
			break
		}
		explicit := false
		if p.peekOperator("AND") {
			p.i += 3
			explicit = true
		}
		child, ok := p.parseUnary()
		if !ok {
			if explicit {
				continue
			}
			return nil, false
		}
		parts = append(parts, child)
	}
	switch len(parts) {
	case 0:
		return nil, true
	case 1:
		return parts[0], true
	default:
		return &bqNode{kind: bqAnd, children: parts}, true
	}
}

func (p *bqParser) parseUnary() (*bqNode, bool) {
	p.skipSpace()
	if p.i >= len(p.src) {
		return nil, false
	}
	if p.src[p.i] == '-' && p.i+1 < len(p.src) && p.src[p.i+1] != ' ' && p.src[p.i+1] != ')' {
		p.i++
		child, ok := p.parseUnary()
		if !ok {
			return nil, false
		}
		return &bqNode{kind: bqNot, children: []*bqNode{child}}, true
	}
	return p.parsePrimary()
}

func (p *bqParser) parsePrimary() (*bqNode, bool) {
	p.skipSpace()
	if p.i >= len(p.src) {
		return nil, false
	}
	c := p.src[p.i]
	switch c {
	case '(':
		p.i++
		child, ok := p.parseOr()
		if !ok {
			return nil, false
		}
		p.skipSpace()
		if p.i >= len(p.src) || p.src[p.i] != ')' {
			return nil, false
		}
		p.i++
		return child, true
	case '"':
		var b strings.Builder
		p.i++
		for p.i < len(p.src) && p.src[p.i] != '"' {
			b.WriteRune(p.src[p.i])
			p.i++
		}
		if p.i >= len(p.src) {
			return nil, false
		}
		p.i++
		return &bqNode{kind: bqTerm, text: b.String()}, true
	case ')':
		return nil, false
	default:
		start := p.i
		for p.i < len(p.src) {
			r := p.src[p.i]
			if r == ' ' || r == '(' || r == ')' || r == '"' || r == '|' {
				break
			}
			p.i++
		}
		word := string(p.src[start:p.i])
		if word == "" {
			return nil, false
		}
		return &bqNode{kind: bqTerm, text: word}, true
	}
}

func renderSearchLeaf(text string) (string, bool) {
	toks := searchTokens(text)
	if len(toks) == 0 {
		return "", false
	}
	for i := range toks {
		toks[i] = strings.ReplaceAll(toks[i], `"`, `""`)
	}
	return `"` + strings.Join(toks, " ") + `"`, true
}

func flattenBQ(n *bqNode, kind bqNodeKind) []*bqNode {
	if n.kind == kind {
		out := make([]*bqNode, 0, 4)
		for _, c := range n.children {
			out = append(out, flattenBQ(c, kind)...)
		}
		return out
	}
	return []*bqNode{n}
}

func renderBQNode(n *bqNode) (string, bool) {
	switch n.kind {
	case bqTerm:
		return renderSearchLeaf(n.text)
	case bqOr:
		var parts []string
		for _, c := range flattenBQ(n, bqOr) {
			if c.kind == bqNot {
				c = c.children[0]
			}
			if s, ok := renderBQNode(c); ok {
				parts = append(parts, s)
			}
		}
		if len(parts) == 0 {
			return "", false
		}
		return "(" + strings.Join(parts, " OR ") + ")", true
	case bqAnd:
		var pos, neg []string
		for _, c := range flattenBQ(n, bqAnd) {
			if c.kind == bqNot {
				if s, ok := renderBQNode(c.children[0]); ok {
					neg = append(neg, s)
				}
				continue
			}
			if s, ok := renderBQNode(c); ok {
				pos = append(pos, s)
			}
		}
		if len(pos) == 0 {
			return "", false
		}
		expr := strings.Join(pos, " AND ")
		if len(neg) > 0 {
			if len(pos) > 1 {
				expr = "(" + expr + ")"
			}
			expr += " NOT (" + strings.Join(neg, " OR ") + ")"
		}
		return expr, true
	case bqNot:
		return "", false
	default:
		return "", false
	}
}

const maxBooleanQueryLen = 512

func sanitizeBooleanInput(raw string) string {
	raw = strings.ToValidUTF8(raw, "")
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range raw {
		if r < 0x20 {
			b.WriteRune(' ')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func buildBooleanFTSQuery(raw string) (string, bool) {
	raw = strings.TrimSpace(sanitizeBooleanInput(raw))
	if raw == "" || len(raw) > maxBooleanQueryLen {
		return "", false
	}
	ast, ok := (&bqParser{src: []rune(raw)}).parseQuery()
	if !ok || ast == nil {
		return "", false
	}
	return renderBQNode(ast)
}

func splitPIDToken(raw string) (*int32, string) {
	var keep []string
	var pid *int32
	for _, tok := range strings.Fields(raw) {
		if pid == nil && strings.HasPrefix(tok, "#") && len(tok) > 1 {
			if v, err := strconv.ParseInt(tok[1:], 10, 32); err == nil {
				p := int32(v)
				pid = &p
				continue
			}
		}
		keep = append(keep, tok)
	}
	return pid, strings.Join(keep, " ")
}
