package report

import (
	"fmt"
	"io"
	"strings"
)

func writeText(w io.Writer, r Report) error {
	if len(r.Findings) == 0 {
		_, err := fmt.Fprintln(w, "No findings.")
		return err
	}

	for _, f := range r.Findings {
		severity := strings.ToUpper(string(f.Severity))
		_, err := fmt.Fprintf(w, "[%s] %s — %s\n", severity, f.Resource, f.Message)
		if err != nil {
			return err
		}
	}

	summary := buildSummary(r.Findings)
	_, err := fmt.Fprintf(w, "\n%d findings (%d high, %d medium, %d low)",
		summary.Total,
		summary.BySeverity["high"],
		summary.BySeverity["medium"],
		summary.BySeverity["low"],
	)
	if err != nil {
		return err
	}

	if summary.EstimatedMonthlyCost > 0 {
		_, err = fmt.Fprintf(w, " — estimated $%.2f/month", summary.EstimatedMonthlyCost)
		if err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(w)
	return err
}
