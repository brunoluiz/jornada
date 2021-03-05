package search

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var metaRegex = regexp.MustCompile(`(meta\.[\w]+)([\s=])+([\d]|['\w])+`)
var quotesRegex = regexp.MustCompile(`'(.*?)'`)
var validRegex = regexp.MustCompile(`^([\w\d\s.='"()><=!/:-])+$`)
var dangerousRegex = regexp.MustCompile(`((--)|([/*])).+`)

// ToSQL parse an input string to a valid SQL string
// TODO: It should be replaced with a v2, with a proper DSL (which I am not currently bothered atm)
func ToSQL(in string) (out string, params []interface{}, err error) {
	if ok := validRegex.MatchString(in); !ok {
		return out, params, errors.New("Invalid query")
	}

	// Replace all " with '
	out = strings.ReplaceAll(in, "\"", "'")

	// Replace all possible dangerous expressions
	out = dangerousRegex.ReplaceAllString(in, "")

	// Transform all `meta.foo = 'bar'` into `meta.key = 'foo' and meta.value = 'bar'`
	out = metaRegex.ReplaceAllStringFunc(out, func(m string) string {
		// strip quote mark
		m = strings.Replace(m, "'", "", -1)
		entries := strings.Split(m, "=")

		ident, value := strings.Split(entries[0], "."), strings.Trim(entries[1], " ")
		key := strings.Trim(ident[1], " ")

		return fmt.Sprintf("json_extract(meta, '$.%s') = '%s'", key, value)
	})

	// Print all strings as SQL placeholders
	out = quotesRegex.ReplaceAllStringFunc(out, func(m string) string {
		m = strings.Replace(m, "'", "", -1)
		m = strings.Trim(m, " ")
		params = append(params, m)
		return "?"
	})

	return strings.Trim(out, " "), params, nil
}
