package chatterbot

import (
	"math"
	"strconv"
	"strings"
	"unicode"
)

// MathematicalEvaluation evaluates arithmetic written with numbers, symbols,
// or English number words.
type MathematicalEvaluation struct{}

// NewMathematicalEvaluation constructs a mathematical evaluation adapter.
func NewMathematicalEvaluation() *MathematicalEvaluation {
	return &MathematicalEvaluation{}
}

// CanProcess reports whether input is a valid mathematical expression.
func (adapter *MathematicalEvaluation) CanProcess(input string) bool {
	_, ok := adapter.Process(input)
	return ok
}

// Process evaluates input when it contains a valid arithmetic expression.
func (*MathematicalEvaluation) Process(input string) (Response, bool) {
	expression := extractExpression(input)
	if expression == "" {
		return Response{}, false
	}
	tokens, hasOperation, ok := mathTokens(expression)
	if !ok || !hasOperation {
		return Response{}, false
	}
	parser := expressionParser{tokens: tokens}
	value, ok := parser.parseExpression()
	if !ok || parser.position != len(tokens) || math.IsInf(value, 0) || math.IsNaN(value) {
		return Response{}, false
	}
	if value == 0 {
		value = 0 // Do not print negative zero.
	}
	result := strconv.FormatFloat(value, 'f', -1, 64)
	return Response{Text: expression + " = " + result, Confidence: 1}, true
}

func extractExpression(input string) string {
	expression := strings.TrimSpace(input)
	expression = strings.TrimRightFunc(expression, func(r rune) bool {
		return unicode.IsSpace(r) || r == '?' || r == '!' || r == '.'
	})
	lower := strings.ToLower(expression)
	prefixes := []string{
		"please calculate ", "please compute ", "how much is ", "what is ",
		"what's ", "whats ", "calculate ", "compute ",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			expression = strings.TrimSpace(expression[len(prefix):])
			break
		}
	}
	return expression
}

type mathTokenKind uint8

const (
	tokenNumber mathTokenKind = iota
	tokenPlus
	tokenMinus
	tokenMultiply
	tokenDivide
	tokenPower
	tokenLeftParen
	tokenRightParen
	tokenPercent
)

type mathToken struct {
	kind  mathTokenKind
	value float64
}

func mathTokens(expression string) ([]mathToken, bool, bool) {
	parts, ok := scanMathParts(strings.ToLower(expression))
	if !ok {
		return nil, false, false
	}
	var tokens []mathToken
	hasOperation := false
	for i := 0; i < len(parts); {
		part := parts[i]
		if value, err := strconv.ParseFloat(strings.ReplaceAll(part, ",", ""), 64); err == nil {
			tokens = append(tokens, mathToken{kind: tokenNumber, value: value})
			i++
			continue
		}
		if value, consumed, found := consumeWordNumber(parts[i:]); found {
			tokens = append(tokens, mathToken{kind: tokenNumber, value: value})
			i += consumed
			continue
		}
		kind, consumed, found := consumeOperator(parts[i:])
		if !found {
			return nil, false, false
		}
		tokens = append(tokens, mathToken{kind: kind})
		if kind != tokenLeftParen && kind != tokenRightParen {
			hasOperation = true
		}
		i += consumed
	}
	return tokens, hasOperation, len(tokens) > 0
}

func scanMathParts(input string) ([]string, bool) {
	runes := []rune(input)
	var parts []string
	for i := 0; i < len(runes); {
		r := runes[i]
		if unicode.IsSpace(r) {
			i++
			continue
		}
		if strings.ContainsRune("+-*/^()%×÷", r) {
			parts = append(parts, string(r))
			i++
			continue
		}
		if runes[i] >= '0' && runes[i] <= '9' || runes[i] == '.' {
			start := i
			dots := 0
			for i < len(runes) && ((runes[i] >= '0' && runes[i] <= '9') || runes[i] == '.' || runes[i] == ',') {
				if runes[i] == '.' {
					dots++
				}
				i++
			}
			if dots > 1 {
				return nil, false
			}
			parts = append(parts, string(runes[start:i]))
			continue
		}
		if unicode.IsLetter(r) || r == '\'' {
			start := i
			for i < len(runes) {
				r = runes[i]
				if !unicode.IsLetter(r) && r != '\'' && r != '-' {
					break
				}
				i++
			}
			parts = append(parts, string(runes[start:i]))
			continue
		}
		return nil, false
	}
	return parts, true
}

var smallNumbers = map[string]float64{
	"zero": 0, "one": 1, "two": 2, "three": 3, "four": 4,
	"five": 5, "six": 6, "seven": 7, "eight": 8, "nine": 9,
	"ten": 10, "eleven": 11, "twelve": 12, "thirteen": 13,
	"fourteen": 14, "fifteen": 15, "sixteen": 16, "seventeen": 17,
	"eighteen": 18, "nineteen": 19,
}

var tensNumbers = map[string]float64{
	"twenty": 20, "thirty": 30, "forty": 40, "fifty": 50,
	"sixty": 60, "seventy": 70, "eighty": 80, "ninety": 90,
}

func consumeWordNumber(parts []string) (float64, int, bool) {
	var total, group float64
	consumed := 0
	seen := false
	lastWasSmall := false
	for consumed < len(parts) {
		word := parts[consumed]
		if strings.Contains(word, "-") {
			pieces := strings.Split(word, "-")
			if len(pieces) == 2 {
				if tens, ok := tensNumbers[pieces[0]]; ok {
					if unit, ok := smallNumbers[pieces[1]]; ok && unit < 10 {
						group += tens + unit
						seen = true
						consumed++
						lastWasSmall = true
						continue
					}
				}
			}
			break
		}
		if value, ok := smallNumbers[word]; ok {
			if lastWasSmall {
				break
			}
			group += value
			seen = true
			consumed++
			lastWasSmall = true
			continue
		}
		if value, ok := tensNumbers[word]; ok {
			group += value
			seen = true
			consumed++
			lastWasSmall = false
			continue
		}
		if word == "hundred" && seen && group > 0 && group < 100 {
			group *= 100
			consumed++
			lastWasSmall = false
			continue
		}
		if (word == "thousand" || word == "million" || word == "billion") && seen && group > 0 {
			scale := float64(1000)
			if word == "million" {
				scale = 1_000_000
			} else if word == "billion" {
				scale = 1_000_000_000
			}
			total += group * scale
			group = 0
			consumed++
			lastWasSmall = false
			continue
		}
		if word == "and" && seen && consumed+1 < len(parts) {
			consumed++
			lastWasSmall = false
			continue
		}
		if word == "point" && seen {
			consumed++
			place := 0.1
			digits := 0
			for consumed < len(parts) {
				digit, ok := smallNumbers[parts[consumed]]
				if !ok || digit >= 10 {
					break
				}
				group += digit * place
				place /= 10
				digits++
				consumed++
			}
			if digits == 0 {
				return 0, 0, false
			}
			break
		}
		break
	}
	return total + group, consumed, seen
}

func consumeOperator(parts []string) (mathTokenKind, int, bool) {
	word := parts[0]
	switch word {
	case "+", "plus", "added":
		return tokenPlus, 1, true
	case "-", "minus", "less", "negative":
		return tokenMinus, 1, true
	case "*", "×", "times", "of":
		return tokenMultiply, 1, true
	case "/", "÷", "over":
		return tokenDivide, 1, true
	case "^":
		return tokenPower, 1, true
	case "(":
		return tokenLeftParen, 1, true
	case ")":
		return tokenRightParen, 1, true
	case "%", "percent", "percentage":
		return tokenPercent, 1, true
	case "multiplied":
		if len(parts) > 1 && parts[1] == "by" {
			return tokenMultiply, 2, true
		}
	case "divided":
		if len(parts) > 1 && parts[1] == "by" {
			return tokenDivide, 2, true
		}
	case "to":
		if len(parts) >= 4 && parts[1] == "the" && parts[2] == "power" && parts[3] == "of" {
			return tokenPower, 4, true
		}
	}
	return 0, 0, false
}

type expressionParser struct {
	tokens   []mathToken
	position int
}

func (p *expressionParser) parseExpression() (float64, bool) {
	left, ok := p.parseProduct()
	for ok && p.position < len(p.tokens) {
		operator := p.tokens[p.position].kind
		if operator != tokenPlus && operator != tokenMinus {
			break
		}
		p.position++
		right, valid := p.parseProduct()
		if !valid {
			return 0, false
		}
		if operator == tokenPlus {
			left += right
		} else {
			left -= right
		}
	}
	return left, ok
}

func (p *expressionParser) parseProduct() (float64, bool) {
	left, ok := p.parsePower()
	for ok && p.position < len(p.tokens) {
		operator := p.tokens[p.position].kind
		if operator == tokenPercent {
			left /= 100
			p.position++
			continue
		}
		if operator != tokenMultiply && operator != tokenDivide {
			break
		}
		p.position++
		right, valid := p.parsePower()
		if !valid || operator == tokenDivide && right == 0 {
			return 0, false
		}
		if operator == tokenMultiply {
			left *= right
		} else {
			left /= right
		}
	}
	return left, ok
}

func (p *expressionParser) parsePower() (float64, bool) {
	left, ok := p.parseUnary()
	if !ok || p.position >= len(p.tokens) || p.tokens[p.position].kind != tokenPower {
		return left, ok
	}
	p.position++
	right, ok := p.parsePower()
	if !ok {
		return 0, false
	}
	return math.Pow(left, right), true
}

func (p *expressionParser) parseUnary() (float64, bool) {
	if p.position < len(p.tokens) && (p.tokens[p.position].kind == tokenPlus || p.tokens[p.position].kind == tokenMinus) {
		negative := p.tokens[p.position].kind == tokenMinus
		p.position++
		value, ok := p.parseUnary()
		if negative {
			value = -value
		}
		return value, ok
	}
	return p.parsePrimary()
}

func (p *expressionParser) parsePrimary() (float64, bool) {
	if p.position >= len(p.tokens) {
		return 0, false
	}
	token := p.tokens[p.position]
	p.position++
	if token.kind == tokenNumber {
		return token.value, true
	}
	if token.kind != tokenLeftParen {
		return 0, false
	}
	value, ok := p.parseExpression()
	if !ok || p.position >= len(p.tokens) || p.tokens[p.position].kind != tokenRightParen {
		return 0, false
	}
	p.position++
	return value, true
}
