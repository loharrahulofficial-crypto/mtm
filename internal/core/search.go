package core

import "strings"

// searchKeys are the key:value tokens understood by both saved views and
// the TUI filter bar. Everything else is treated as free text.
func searchKey(key string) bool {
	switch strings.ToLower(key) {
	case "tag", "project", "status", "priority", "due":
		return true
	}
	return false
}

// ParseSearchQuery splits a filter-bar query into a structured Filter (via
// ParseViewExpr for the key:value tokens) and a free-text needle. Example:
//
//	"tag:exam due:today calculus" -> Filter{Tag:"exam", Due:"today"}, "calculus"
//
// Unknown keys are left in the needle (so "foo:bar" searches for the literal
// text). The returned Filter does not include Now/DoneCheck; the caller sets
// those. An empty query yields the zero Filter and no needle.
func ParseSearchQuery(query string) (Filter, string, error) {
	var keys []string
	var needle []string
	for _, tok := range strings.Fields(query) {
		key, _, ok := strings.Cut(tok, ":")
		if ok && searchKey(key) {
			keys = append(keys, tok)
			continue
		}
		needle = append(needle, tok)
	}
	f, err := ParseViewExpr(strings.Join(keys, " "))
	if err != nil {
		return f, "", err
	}
	f.Contains = strings.Join(needle, " ")
	return f, f.Contains, nil
}
