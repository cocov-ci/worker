package runner

import (
	"bytes"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestParseReportItems(t *testing.T) {
	item1 := ReportItem{
		Kind:      "kind",
		File:      "file",
		LineStart: 1,
		LineEnd:   2,
		Message:   "mess",
		UID:       "uid",
	}
	item2 := ReportItem{
		Kind:      "kind2",
		File:      "file",
		LineStart: 1,
		LineEnd:   2,
		Message:   "mess",
		UID:       "uid",
	}

	j1, err := json.Marshal(item1)
	require.NoError(t, err)
	j2, err := json.Marshal(item2)
	require.NoError(t, err)
	b := bytes.Buffer{}
	b.Write(j1)
	b.WriteByte(0x00)

	list, err := ParseReportItems(b.Bytes())
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "kind", list[0].Kind)

	b.Write(j2)
	b.WriteByte(0x00)

	list, err = ParseReportItems(b.Bytes())
	require.NoError(t, err)
	assert.Len(t, list, 2)
	assert.Equal(t, "kind", list[0].Kind)
	assert.Equal(t, "kind2", list[1].Kind)
}

func TestParseReportItemsEmpty(t *testing.T) {
	list, err := ParseReportItems([]byte(""))
	assert.NoError(t, err)
	assert.Empty(t, list)
}
