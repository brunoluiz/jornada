package search_test

import (
	"testing"

	"github.com/brunoluiz/jornada/internal/search/v1"
	"github.com/stretchr/testify/require"
)

func TestTransform(t *testing.T) {
	tests := []struct {
		in     string
		out    string
		params []interface{}
		err    bool
	}{
		{
			in:     "meta.foo = 'bar' AND meta.x = 'y'",
			out:    "(meta.key = ? AND meta.value = ?) AND (meta.key = ? AND meta.value = ?)",
			params: []interface{}{"foo", "bar", "x", "y"},
		},
		{
			in:  "meta.foo = ';;bar' AND meta.x = 'y'",
			err: true,
		},
		{
			in:     "(meta.foo = 'bar' AND meta.x = 'y') OR value = '1'",
			out:    "((meta.key = ? AND meta.value = ?) AND (meta.key = ? AND meta.value = ?)) OR value = ?",
			params: []interface{}{"foo", "bar", "x", "y", "1"},
		},
		{
			in:     "(meta.foo = 10 AND meta.x = 'y') OR value = '1'",
			out:    "((meta.key = ? AND meta.value = ?) AND (meta.key = ? AND meta.value = ?)) OR value = ?",
			params: []interface{}{"foo", "10", "x", "y", "1"},
		},
		{
			in:     "meta.test = 'x' -- comment",
			out:    "(meta.key = ? AND meta.value = ?)",
			params: []interface{}{"test", "x"},
		},
	}

	for _, test := range tests {
		out, params, err := search.ToSQL(test.in)
		require.Equal(t, test.err, err != nil)
		require.Equal(t, test.out, out)
		require.Equal(t, test.params, params)
	}
}
