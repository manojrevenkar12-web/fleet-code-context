// Package words turns identifiers and prose into comparable terms.
package words

import (
	"strings"
	"unicode"
)

var stop = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`a an and are as at be but by can do does for from has have how i if in into is it its
	of on or our should so than that the their them then there these they this to too was we were what when where which
	who why will with would you your not no all any each new now get set add make use only also more must after before
	shown show see sure same one two being been very just like want need there's it's don't doesn't currently instead
	ticket customers customer`) {
		stop[w] = true
	}
}

// Split breaks an identifier or sentence into lower-case terms: "GetEtaRequest" -> get, eta, request;
// "eta_seconds" -> eta, seconds. Stop words and one-letter terms are dropped; terms are lightly stemmed.
func Split(s string) []string {
	var out []string
	var cur []rune
	flush := func() {
		if len(cur) > 1 {
			w := Stem(strings.ToLower(string(cur)))
			if !stop[w] && len(w) > 1 {
				out = append(out, w)
			}
		}
		cur = cur[:0]
	}
	rs := []rune(s)
	for i, r := range rs {
		switch {
		case unicode.IsLetter(r):
			if unicode.IsUpper(r) && len(cur) > 0 {
				prevLower := unicode.IsLower(rs[i-1])
				nextLower := i+1 < len(rs) && unicode.IsLower(rs[i+1])
				if prevLower || (nextLower && unicode.IsUpper(rs[i-1])) {
					flush()
				}
			}
			cur = append(cur, r)
		case unicode.IsDigit(r):
			cur = append(cur, r)
		default:
			flush()
		}
	}
	flush()
	return out
}

// Stem removes common English suffixes so "refunds", "refunded" and "refund" meet.
func Stem(w string) string {
	for _, suf := range []string{"ing", "ed", "es", "s"} {
		if len(w) > len(suf)+3 && strings.HasSuffix(w, suf) {
			base := strings.TrimSuffix(w, suf)
			if suf == "es" && !strings.HasSuffix(base, "ss") && !strings.HasSuffix(base, "x") && !strings.HasSuffix(base, "ch") {
				return base + "e"
			}
			return base
		}
	}
	return w
}

// Set returns the distinct terms of s.
func Set(s string) map[string]bool {
	m := map[string]bool{}
	for _, w := range Split(s) {
		m[w] = true
	}
	return m
}

// Tokens estimates how many model tokens a text costs (4 characters a token, the usual rule for code).
func Tokens(s string) int { return (len(s) + 3) / 4 }
