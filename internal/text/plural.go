// Package text holds tiny shared string helpers.
package text

import "fmt"

// Plural picks the singular form when n == 1, else the plural.
//
//	text.Plural(n, "day", "days")
func Plural(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// Count renders "N <noun>" with the noun inflected: "1 day", "3 days".
func Count(n int, singular, plural string) string {
	return fmt.Sprintf("%d %s", n, Plural(n, singular, plural))
}
