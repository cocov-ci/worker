package runner

import (
	"bytes"
	"encoding/json"
)

type ReportItem struct {
	Kind      string `json:"kind,omitempty"`
	File      string `json:"file,omitempty"`
	LineStart int    `json:"line_start,omitempty"`
	LineEnd   int    `json:"line_end,omitempty"`
	Message   string `json:"message,omitempty"`
	UID       string `json:"uid,omitempty"`
}

func ParseReportItems(data []byte) ([]ReportItem, error) {
	if len(data) == 0 {
		return nil, nil
	}

	data = bytes.TrimSuffix(data, []byte{0x00})
	records := bytes.Split(data, []byte{0x00})
	items := make([]ReportItem, len(records))

	for i, record := range records {
		if err := json.Unmarshal(record, &items[i]); err != nil {
			return nil, err
		}
	}

	return items, nil
}
