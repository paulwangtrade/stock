package opportunity_test

import (
	"testing"

	"go-stock/backend/opportunity"

	"github.com/stretchr/testify/require"
)

func TestNormalizeReadStockCode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"301125.SZ", "sz301125"},
		{"301125.sz", "sz301125"},
		{"sz301125", "sz301125"},
		{"SZ301125", "sz301125"},
		{"301125", "sz301125"},
		{"600363.SH", "sh600363"},
		{"", ""},
		{"  ", ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, opportunity.NormalizeReadStockCode(tc.in))
		})
	}
}
