package data

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitRealtimeFetchChunks(t *testing.T) {
	codes := make([]string, 65)
	for i := range codes {
		codes[i] = "c" + strconv.Itoa(i)
	}
	require.Nil(t, splitRealtimeFetchChunks(codes, 80, 30))

	chunks := splitRealtimeFetchChunks(codes, 50, 30)
	require.Len(t, chunks, 3)
	require.Len(t, chunks[0], 30)
	require.Len(t, chunks[1], 30)
	require.Len(t, chunks[2], 5)
}
