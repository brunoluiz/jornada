package search_test

import (
	"testing"

	"github.com/brunoluiz/jornada/internal/search"
	"github.com/stretchr/testify/require"
)

func TestTransform(t *testing.T) {
	tests := []struct {
		in     string
		out    string
		params []interface{}
	}{
		{
			in:     "meta.foo = 'bar' AND meta.x = 'y'",
			out:    "(meta.key = $1 AND meta.value = $2) AND (meta.key = $3 AND meta.value = $4)",
			params: []interface{}{"foo", "bar", "x", "y"},
		},
		{
			in:     "meta.foo = ';;bar' AND meta.x = 'y'",
			out:    "(meta.key = $1 AND meta.value = $2) AND (meta.key = $3 AND meta.value = $4)",
			params: []interface{}{"foo", "bar", "x", "y"},
		},
		{
			in:     "(meta.foo = 'bar' AND meta.x = 'y') OR value = '1'",
			out:    "((meta.key = $1 AND meta.value = $2) AND (meta.key = $3 AND meta.value = $4)) OR value = $5",
			params: []interface{}{"foo", "bar", "x", "y", "1"},
		},
	}

	for _, test := range tests {
		out, params, err := search.Transform(test.in)
		require.NoError(t, err)
		require.Equal(t, out, test.out)
		require.Equal(t, params, test.params)
	}
}
