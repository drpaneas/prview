package analyzer

import (
	"fmt"
	"sort"

	"github.com/drpaneas/prview/internal/model"
)

func Analyze(data *model.PRData, aiAnalysis *model.AIAnalysis) *model.PRReport {
	scope := AnalyzeScope(data.Diffs)
	classify := ClassifyChange(data.Diffs, data.Meta)
	astResult := AnalyzeAST(data.BaseFiles, data.HeadFiles)
	risks := DetectRisks(data.Diffs, data.HeadFiles)
	reviewers := SuggestReviewers(data.Blames, data.Meta.Author)
	arch := AnalyzeArchitecture(data.Diffs, data.BaseFiles, data.HeadFiles, astResult)

	focus := buildReviewFocus(risks, scope)
	verdict, note := computeVerdict(risks, scope)

	if aiAnalysis != nil && aiAnalysis.Verdict != "" {
		switch aiAnalysis.Verdict {
		case "approve":
			verdict = model.VerdictApprove
		case "request_changes":
			verdict = model.VerdictRequestChanges
		case "discuss":
			verdict = model.VerdictDiscuss
		}
		note = aiAnalysis.VerdictReason
	}

	return &model.PRReport{
		Meta:         data.Meta,
		Scope:        scope,
		Classify:     classify,
		AST:          astResult,
		Risks:        risks,
		Reviewers:    reviewers,
		Architecture: arch,
		Author:       data.Author,
		ReviewFocus:  focus,
		Verdict:      verdict,
		VerdictNote:  note,
		AI:           aiAnalysis,
	}
}

// PreComputeRisks runs the heuristic risk detection before the AI call
// so the risks can be passed to the AI for commentary.
func PreComputeRisks(data *model.PRData) []model.RiskFlag {
	return DetectRisks(data.Diffs, data.HeadFiles)
}

func buildReviewFocus(risks []model.RiskFlag, scope model.ScopeResult) []model.ReviewFocus {
	fileRisks := map[string][]model.RiskFlag{}
	for _, r := range risks {
		fileRisks[r.File] = append(fileRisks[r.File], r)
	}

	type fileScore struct {
		file  string
		score int
	}
	var scored []fileScore

	for file, rr := range fileRisks {
		s := 0
		for _, r := range rr {
			switch r.Severity {
			case model.SeverityHigh:
				s += 3
			case model.SeverityMedium:
				s += 2
			case model.SeverityLow:
				s += 1
			}
		}
		scored = append(scored, fileScore{file: file, score: s})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	if len(scored) > 5 {
		scored = scored[:5]
	}

	var focus []model.ReviewFocus
	for _, s := range scored {
		priority := "low"
		if s.score >= 6 {
			priority = "critical"
		} else if s.score >= 3 {
			priority = "high"
		} else if s.score >= 2 {
			priority = "medium"
		}

		var lookFor []string
		for _, r := range fileRisks[s.file] {
			lookFor = append(lookFor, fmt.Sprintf("%s (line %d)", r.Description, r.Line))
		}

		why := fmt.Sprintf("%d risk flag(s)", len(fileRisks[s.file]))

		focus = append(focus, model.ReviewFocus{
			File:     s.file,
			Priority: priority,
			Why:      why,
			LookFor:  lookFor,
		})
	}

	return focus
}

func computeVerdict(risks []model.RiskFlag, scope model.ScopeResult) (model.Verdict, string) {
	highRisks := 0
	medRisks := 0
	for _, r := range risks {
		switch r.Severity {
		case model.SeverityHigh:
			highRisks++
		case model.SeverityMedium:
			medRisks++
		}
	}

	if highRisks >= 2 {
		return model.VerdictRequestChanges, fmt.Sprintf(
			"%d high-risk issue(s) detected",
			highRisks,
		)
	}

	if highRisks >= 1 || medRisks >= 3 {
		return model.VerdictDiscuss, fmt.Sprintf(
			"%d risk flag(s) - review carefully",
			highRisks+medRisks,
		)
	}

	return model.VerdictApprove, "no major issues detected"
}
