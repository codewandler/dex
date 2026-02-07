package homer

import (
	"fmt"
	"strings"
	"unicode"
)

// queryFields maps user-friendly field names to Homer's internal column names.
var queryFields = map[string]string{
	"from_user":  "data_header.from_user",
	"to_user":    "data_header.to_user",
	"ruri_user":  "data_header.ruri_user",
	"user_agent": "data_header.user_agent",
	"ua":         "data_header.user_agent",
	"cseq":       "data_header.cseq",
	"method":     "method",
	"status":     "status",
	"call_id":    "sid",
	"sid":        "sid",
}

// tokenType represents the type of a lexer token.
type tokenType int

const (
	tokIdent  tokenType = iota // identifier (field name)
	tokString                  // single-quoted string
	tokNumber                  // numeric literal
	tokEq                     // =
	tokNeq                    // !=
	tokLParen                 // (
	tokRParen                 // )
	tokAnd                    // AND
	tokOr                     // OR
	tokEOF                    // end of input
)

type token struct {
	typ tokenType
	val string
	pos int
}

// tokenize splits input into tokens.
func tokenize(input string) ([]token, error) {
	var tokens []token
	i := 0

	for i < len(input) {
		// Skip whitespace
		if unicode.IsSpace(rune(input[i])) {
			i++
			continue
		}

		switch {
		case input[i] == '(':
			tokens = append(tokens, token{tokLParen, "(", i})
			i++
		case input[i] == ')':
			tokens = append(tokens, token{tokRParen, ")", i})
			i++
		case input[i] == '=' :
			tokens = append(tokens, token{tokEq, "=", i})
			i++
		case input[i] == '!' && i+1 < len(input) && input[i+1] == '=':
			tokens = append(tokens, token{tokNeq, "!=", i})
			i += 2
		case input[i] == '\'':
			// Quoted string
			start := i
			i++ // skip opening quote
			var sb strings.Builder
			for i < len(input) && input[i] != '\'' {
				sb.WriteByte(input[i])
				i++
			}
			if i >= len(input) {
				return nil, fmt.Errorf("unterminated string at position %d", start)
			}
			i++ // skip closing quote
			tokens = append(tokens, token{tokString, sb.String(), start})
		case input[i] >= '0' && input[i] <= '9':
			start := i
			for i < len(input) && input[i] >= '0' && input[i] <= '9' {
				i++
			}
			tokens = append(tokens, token{tokNumber, input[start:i], start})
		case isIdentStart(input[i]):
			start := i
			for i < len(input) && isIdentChar(input[i]) {
				i++
			}
			word := input[start:i]
			switch strings.ToUpper(word) {
			case "AND":
				tokens = append(tokens, token{tokAnd, "AND", start})
			case "OR":
				tokens = append(tokens, token{tokOr, "OR", start})
			default:
				tokens = append(tokens, token{tokIdent, word, start})
			}
		default:
			return nil, fmt.Errorf("unexpected character %q at position %d", input[i], i)
		}
	}

	tokens = append(tokens, token{tokEOF, "", len(input)})
	return tokens, nil
}

func isIdentStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

func isIdentChar(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9') || c == '.'
}

// condition is an intermediate representation of a parsed expression node.
type condition struct {
	// leaf
	field string // mapped Homer field name
	op    string // "=" or "!="
	value string // literal value (string or number)
	isNum bool   // true if value is numeric

	// composite
	logic    string       // "AND" or "OR" (empty for leaf)
	children []*condition // sub-conditions
}

// toSmartInput renders a condition tree as a Homer smart input string.
func (c *condition) toSmartInput() string {
	if c.logic != "" {
		parts := make([]string, len(c.children))
		for i, ch := range c.children {
			s := ch.toSmartInput()
			// Wrap child in parens if it's a composite with different logic
			if ch.logic != "" && ch.logic != c.logic {
				s = "(" + s + ")"
			}
			parts[i] = s
		}
		return strings.Join(parts, " "+c.logic+" ")
	}

	// Leaf node
	if c.isNum {
		return fmt.Sprintf("%s %s %s", c.field, c.op, c.value)
	}
	return fmt.Sprintf("%s %s '%s'", c.field, c.op, c.value)
}

// parser holds state for recursive descent parsing.
type parser struct {
	tokens []token
	pos    int
}

func (p *parser) peek() token {
	return p.tokens[p.pos]
}

func (p *parser) advance() token {
	t := p.tokens[p.pos]
	p.pos++
	return t
}

func (p *parser) expect(typ tokenType) (token, error) {
	t := p.peek()
	if t.typ != typ {
		return t, fmt.Errorf("expected %s at position %d, got %q", tokenName(typ), t.pos, t.val)
	}
	return p.advance(), nil
}

// parseExpr parses: condition ((AND | OR) condition)*
func (p *parser) parseExpr() (*condition, error) {
	left, err := p.parseCondition()
	if err != nil {
		return nil, err
	}

	for p.peek().typ == tokAnd || p.peek().typ == tokOr {
		op := p.advance()
		logic := op.val

		right, err := p.parseCondition()
		if err != nil {
			return nil, err
		}

		// Flatten same-level logic
		if left.logic == logic {
			left.children = append(left.children, right)
		} else {
			left = &condition{
				logic:    logic,
				children: []*condition{left, right},
			}
		}
	}

	return left, nil
}

// parseCondition parses: '(' expr ')' | field op value
func (p *parser) parseCondition() (*condition, error) {
	if p.peek().typ == tokLParen {
		p.advance() // consume '('
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokRParen); err != nil {
			return nil, fmt.Errorf("missing closing parenthesis at position %d", p.peek().pos)
		}
		return expr, nil
	}

	// field op value
	fieldTok, err := p.expect(tokIdent)
	if err != nil {
		return nil, fmt.Errorf("expected field name at position %d, got %q", p.peek().pos, p.peek().val)
	}

	mapped, ok := queryFields[fieldTok.val]
	if !ok {
		return nil, fmt.Errorf("unknown field %q at position %d (available: %s)", fieldTok.val, fieldTok.pos, availableFields())
	}

	// Operator
	opTok := p.peek()
	if opTok.typ != tokEq && opTok.typ != tokNeq {
		return nil, fmt.Errorf("expected operator (= or !=) at position %d, got %q", opTok.pos, opTok.val)
	}
	p.advance()

	// Value
	valTok := p.peek()
	if valTok.typ != tokString && valTok.typ != tokNumber {
		return nil, fmt.Errorf("expected value (string or number) at position %d, got %q", valTok.pos, valTok.val)
	}
	p.advance()

	return &condition{
		field: mapped,
		op:    opTok.val,
		value: valTok.val,
		isNum: valTok.typ == tokNumber,
	}, nil
}

// ParseQuery parses a user query string and returns the Homer smart input equivalent.
// Field names are validated and mapped to Homer's internal column names.
// Returns an error for unknown fields or invalid syntax.
func ParseQuery(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil
	}

	tokens, err := tokenize(input)
	if err != nil {
		return "", err
	}

	p := &parser{tokens: tokens}
	cond, err := p.parseExpr()
	if err != nil {
		return "", err
	}

	// Ensure all input was consumed
	if p.peek().typ != tokEOF {
		t := p.peek()
		return "", fmt.Errorf("unexpected token %q at position %d", t.val, t.pos)
	}

	return cond.toSmartInput(), nil
}

func tokenName(t tokenType) string {
	switch t {
	case tokIdent:
		return "identifier"
	case tokString:
		return "string"
	case tokNumber:
		return "number"
	case tokEq:
		return "'='"
	case tokNeq:
		return "'!='"
	case tokLParen:
		return "'('"
	case tokRParen:
		return "')'"
	case tokAnd:
		return "AND"
	case tokOr:
		return "OR"
	case tokEOF:
		return "end of input"
	default:
		return "unknown"
	}
}

func availableFields() string {
	// Deduplicate (aliases map to the same target)
	seen := make(map[string]bool)
	var fields []string
	for name := range queryFields {
		if !seen[name] {
			fields = append(fields, name)
			seen[name] = true
		}
	}
	return strings.Join(fields, ", ")
}
