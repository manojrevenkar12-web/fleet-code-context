// Package money holds amounts as integer pence.
package money

import "fmt"

// Pence is an amount of money in pence.
type Pence int64

// Format renders an amount for a person, e.g. "£12.5".
func Format(p Pence) string {
	pounds := p / 100
	rest := p % 100
	if rest%10 == 0 {
		return fmt.Sprintf("£%d.%d", pounds, rest/10)
	}
	return fmt.Sprintf("£%d.%02d", pounds, rest)
}

// ApplyBasisPoints returns p * bps / 10000, rounded half up.
func ApplyBasisPoints(p Pence, bps int64) Pence {
	return Pence((int64(p)*bps + 5000) / 10000)
}
