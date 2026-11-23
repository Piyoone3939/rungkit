// Package ldt reads and writes the ladder text format (.ldt).
//
//	profile melsec-iqr
//	sym X0 "運転ボタン"
//	rung "運転 自己保持" {
//	  (X0 | Y0) & !X1 -> OUT(Y0)
//	}
//
// & is series, | is parallel (& binds tighter), ! is a b-contact,
// rise(X)/fall(X) are edge contacts, [A >= B] is a comparison, and the right
// side of -> holds outputs. A { } block lists output branches, one per item.
package ldt

import (
	"fmt"
	"strconv"
	"unicode/utf8"
)

// Pos is a 1-based position in the source.
type Pos struct{ Line, Col int }

func (p Pos) String() string { return fmt.Sprintf("%d:%d", p.Line, p.Col) }

type tokKind int

const (
	tEOF tokKind = iota
	tWord
	tString
	tLParen
	tRParen
	tLBrace
	tRBrace
	tLBrack
	tRBrack
	tComma
	tAnd
	tOr
	tNot
	tArrow
	tColon
	tAt
	tCmp
	tSemi
)

var tokNames = map[tokKind]string{
	tEOF: "ファイルの終わり", tWord: "名前", tString: "文字列", tLParen: "(", tRParen: ")",
	tLBrace: "{", tRBrace: "}", tLBrack: "[", tRBrack: "]", tComma: ",", tAnd: "&",
	tOr: "|", tNot: "!", tArrow: "->", tColon: ":", tAt: "@", tCmp: "比較演算子", tSemi: ";",
}

type token struct {
	kind tokKind
	text string
	pos  Pos
}

// Error is a syntax error with its position.
type Error struct {
	Pos Pos
	Msg string
}

func (e *Error) Error() string { return e.Pos.String() + ": " + e.Msg }

func isWordStart(c byte) bool {
	return c == '_' || c == '%' || c == '-' || c == '.' ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func isWordChar(c byte) bool { return isWordStart(c) || c == '#' }

func lex(src string) ([]token, error) {
	var toks []token
	line, col := 1, 1
	i := 0
	advance := func(n int) {
		for k := 0; k < n; k++ {
			if src[i] == '\n' {
				line++
				col = 1
			} else if src[i]&0xC0 != 0x80 { // count runes, not bytes
				col++
			}
			i++
		}
	}
	for i < len(src) {
		c := src[i]
		pos := Pos{line, col}
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			advance(1)
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				advance(1)
			}
		case c == '-' && i+1 < len(src) && src[i+1] == '>':
			toks = append(toks, token{tArrow, "->", pos})
			advance(2)
		case c == '"':
			j := i + 1
			for j < len(src) && src[j] != '"' {
				if src[j] == '\\' {
					j++
				}
				if j < len(src) && src[j] == '\n' {
					return nil, &Error{pos, "文字列が閉じていません"}
				}
				j++
			}
			if j >= len(src) {
				return nil, &Error{pos, "文字列が閉じていません"}
			}
			s, err := strconv.Unquote(src[i : j+1])
			if err != nil {
				return nil, &Error{pos, "文字列の書き方が正しくありません"}
			}
			toks = append(toks, token{tString, s, pos})
			advance(j + 1 - i)
		case c == '<' || c == '>' || c == '=':
			op := string(c)
			if i+1 < len(src) {
				two := src[i : i+2]
				if two == "<=" || two == ">=" || two == "<>" {
					op = two
				}
			}
			toks = append(toks, token{tCmp, op, pos})
			advance(len(op))
		case isWordStart(c):
			j := i
			for j < len(src) && isWordChar(src[j]) {
				if src[j] == '-' && j+1 < len(src) && src[j+1] == '>' {
					break
				}
				j++
			}
			toks = append(toks, token{tWord, src[i:j], pos})
			advance(j - i)
		default:
			k, ok := map[byte]tokKind{
				'(': tLParen, ')': tRParen, '{': tLBrace, '}': tRBrace, '[': tLBrack,
				']': tRBrack, ',': tComma, '&': tAnd, '|': tOr, '!': tNot, ':': tColon,
				'@': tAt, ';': tSemi,
			}[c]
			if !ok {
				r, _ := utf8.DecodeRuneInString(src[i:])
				return nil, &Error{pos, fmt.Sprintf("使えない文字 %q があります", r)}
			}
			toks = append(toks, token{k, string(c), pos})
			advance(1)
		}
	}
	toks = append(toks, token{tEOF, "", Pos{line, col}})
	return toks, nil
}
