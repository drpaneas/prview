package analyzer

import (
	"fmt"
	"sort"
	"time"

	"github.com/drpaneas/prview/internal/model"
)

type ownerScore struct {
	Login      string
	Lines      int
	Files      map[string]bool
	LastActive time.Time
}

func SuggestReviewers(blames []model.BlameResult, prAuthor string) []model.Reviewer {
	scores := map[string]*ownerScore{}
	totalLines := 0

	for _, b := range blames {
		for _, line := range b.Lines {
			if line.Author == "" || line.Email == "" {
				continue
			}
			totalLines++
			key := line.Email
			if _, ok := scores[key]; !ok {
				login := line.Login
				if login == "" {
					login = line.Author
				}
				scores[key] = &ownerScore{
					Login: login,
					Files: map[string]bool{},
				}
			}
			s := scores[key]
			s.Lines++
			s.Files[b.Path] = true
			if line.Date.After(s.LastActive) {
				s.LastActive = line.Date
			}
		}
	}

	// Remove PR author from suggestions (check both login and display name)
	for key, s := range scores {
		if s.Login == prAuthor {
			delete(scores, key)
		}
	}

	var ranked []*ownerScore
	for _, s := range scores {
		ranked = append(ranked, s)
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Lines > ranked[j].Lines
	})

	// Top 3
	if len(ranked) > 3 {
		ranked = ranked[:3]
	}

	var reviewers []model.Reviewer
	for _, s := range ranked {
		ownership := 0.0
		if totalLines > 0 {
			ownership = float64(s.Lines) / float64(totalLines) * 100
		}

		var files []string
		for f := range s.Files {
			files = append(files, f)
		}

		confidence := "low"
		if ownership > 50 {
			confidence = "high"
		} else if ownership > 20 {
			confidence = "medium"
		}

		lastActive := formatTimeAgo(s.LastActive)

		reviewers = append(reviewers, model.Reviewer{
			Login:      s.Login,
			Confidence: confidence,
			Ownership:  ownership,
			Files:      files,
			Reason:     fmt.Sprintf("owns %.0f%% of changed lines across %d file(s)", ownership, len(files)),
			LastActive: lastActive,
		})
	}

	return reviewers
}

func formatTimeAgo(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t)
	switch {
	case d < 24*time.Hour:
		return "today"
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%d weeks ago", int(d.Hours()/(24*7)))
	default:
		return fmt.Sprintf("%d months ago", int(d.Hours()/(24*30)))
	}
}
