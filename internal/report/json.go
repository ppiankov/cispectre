package report

import (
	"encoding/json"
	"io"
)

func writeJSON(w io.Writer, r Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r.Findings)
}
