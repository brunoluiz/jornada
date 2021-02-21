package search

import (
	"fmt"
	"log"
	"regexp"
	"strings"
)

var metaRegex = regexp.MustCompile(`(meta\.[^=]+=[^']'[^\']+')`)
var quotesRegex = regexp.MustCompile(`'(.*?)'`)

func Transform(in string) (string, []interface{}, error) {
	out := strings.Replace(in, "\"", "'", -1)
	params := []interface{}{}

	out = metaRegex.ReplaceAllStringFunc(out, func(m string) string {
		m = strings.Replace(m, "'", "", -1)
		entries := strings.Split(m, "=")

		sub, value := entries[0], entries[1]
		subsplit := strings.Split(sub, ".")
		key := subsplit[1]

		return fmt.Sprintf("(meta.key = '%s' AND meta.value = '%s')", key, value)
	})

	out = quotesRegex.ReplaceAllStringFunc(out, func(m string) string {
		m = strings.Replace(m, "'", "", -1)
		m = strings.Trim(m, " ")
		params = append(params, m)
		return fmt.Sprintf("$%d", len(params))
	})

	log.Println(out)

	return out, params, nil
}
