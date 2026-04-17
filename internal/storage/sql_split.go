package storage

import "strings"

// splitSQLStatements decoupe un dump SQL en statements individuels en
// respectant les separateurs qui peuvent apparaitre a l'interieur de :
//
//   - string literals simples ('...') — doublage pour escape ('')
//   - identifier quotes ("..." ou `...`)
//   - commentaires de ligne (-- jusqu'a fin de ligne)
//   - commentaires de bloc (/* ... */)
//
// Bien plus robuste que strings.Split(";") qui casse des qu'une de ces
// constructions contient un `;`. On s'est fait avoir en P49 avec un
// commentaire francais "le code a ete supprime ; les tables restaient".
//
// N'essaie PAS de parser la grammaire SQL — juste suffisamment intelligent
// pour honorer les delimiteurs. Ne supporte pas les delimiters custom
// (type DELIMITER // de MySQL) — inutile pour les migrations Hive.
func splitSQLStatements(src string) []string {
	var out []string
	var cur strings.Builder

	type stateKind int
	const (
		normal stateKind = iota
		inSingleQuote
		inDoubleQuote
		inBacktick
		inLineComment
		inBlockComment
	)
	state := normal

	for i := 0; i < len(src); i++ {
		c := src[i]
		peek := byte(0)
		if i+1 < len(src) {
			peek = src[i+1]
		}

		switch state {
		case normal:
			switch {
			case c == '-' && peek == '-':
				cur.WriteByte(c)
				cur.WriteByte(peek)
				i++
				state = inLineComment
			case c == '/' && peek == '*':
				cur.WriteByte(c)
				cur.WriteByte(peek)
				i++
				state = inBlockComment
			case c == '\'':
				cur.WriteByte(c)
				state = inSingleQuote
			case c == '"':
				cur.WriteByte(c)
				state = inDoubleQuote
			case c == '`':
				cur.WriteByte(c)
				state = inBacktick
			case c == ';':
				// Vrai terminateur : on flush et on continue.
				out = append(out, cur.String())
				cur.Reset()
			default:
				cur.WriteByte(c)
			}

		case inLineComment:
			cur.WriteByte(c)
			if c == '\n' {
				state = normal
			}

		case inBlockComment:
			cur.WriteByte(c)
			if c == '*' && peek == '/' {
				cur.WriteByte(peek)
				i++
				state = normal
			}

		case inSingleQuote:
			cur.WriteByte(c)
			if c == '\'' {
				// Escape double-apostrophe : 'it''s fine' reste string.
				if peek == '\'' {
					cur.WriteByte(peek)
					i++
					continue
				}
				state = normal
			}

		case inDoubleQuote:
			cur.WriteByte(c)
			if c == '"' {
				state = normal
			}

		case inBacktick:
			cur.WriteByte(c)
			if c == '`' {
				state = normal
			}
		}
	}
	// Dernier statement sans ; final.
	if s := strings.TrimSpace(cur.String()); s != "" {
		out = append(out, cur.String())
	}
	return out
}
