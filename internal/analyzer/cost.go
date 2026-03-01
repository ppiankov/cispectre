package analyzer

import (
	"strings"

	"github.com/ppiankov/cispectre/internal/github"
)

// GitHub Actions per-minute rates (USD).
const (
	RateLinux   = 0.008
	RateWindows = 0.016
	RateMacOS   = 0.08
)

// runnerRate returns the per-minute rate based on job labels.
// Defaults to Linux rate when OS cannot be determined.
func runnerRate(labels []string) float64 {
	for _, l := range labels {
		lower := strings.ToLower(l)
		if strings.Contains(lower, "macos") || strings.Contains(lower, "mac") {
			return RateMacOS
		}
		if strings.Contains(lower, "windows") {
			return RateWindows
		}
	}
	return RateLinux
}

// estimateMonthlyCost computes the estimated monthly cost for a set of runs.
// months is the observation period in months (must be > 0).
func estimateMonthlyCost(runs []github.Run, jobs map[int64][]github.Job, months float64) float64 {
	if months <= 0 || len(runs) == 0 {
		return 0
	}

	var totalCost float64
	for _, r := range runs {
		if r.Status != "completed" {
			continue
		}
		runJobs := jobs[r.ID]
		if len(runJobs) > 0 {
			for _, j := range runJobs {
				durationMin := float64(j.DurationSecs) / 60
				totalCost += durationMin * runnerRate(j.Labels)
			}
		} else {
			// No job data — fall back to run duration with Linux rate
			durationMin := float64(r.DurationSecs) / 60
			totalCost += durationMin * RateLinux
		}
	}

	return totalCost / months
}
