package postgresql

import "strings"

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// EscapeLike makes s match itself literally inside a LIKE pattern (default escape character).
func EscapeLike(s string) string {
	return likeEscaper.Replace(s)
}
