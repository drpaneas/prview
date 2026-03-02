package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/drpaneas/prview/internal/model"
)

func renderReport(r *model.PRReport, width int) string {
	var sb strings.Builder

	sb.WriteString(renderAISlop(r))
	sb.WriteString(renderHeader(r, width))
	sb.WriteString("\n")
	sb.WriteString(renderScope(r))
	sb.WriteString("\n")
	sb.WriteString(renderAISummary(r))
	sb.WriteString("\n")
	sb.WriteString(renderBeforeAfter(r))
	sb.WriteString("\n")
	sb.WriteString(renderAIIssues(r))
	sb.WriteString("\n")
	sb.WriteString(renderReviewQuestions(r))
	sb.WriteString("\n")
	sb.WriteString(renderTestAssessment(r))
	sb.WriteString("\n")
	sb.WriteString(renderRisks(r))
	sb.WriteString("\n")
	sb.WriteString(renderReviewers(r))
	sb.WriteString("\n")
	sb.WriteString(renderPersonaReview(r))
	sb.WriteString(renderVerdict(r))

	return sb.String()
}

func renderAISlop(r *model.PRReport) string {
	if r.AISlop == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(sectionTitleStyle.Render(" AI-GENERATED PR CHECK"))
	sb.WriteString("\n")

	switch r.AISlop.Verdict {
	case "human":
		sb.WriteString(fmt.Sprintf("  %s\n", greenText.Render("HUMAN-WRITTEN")))
	case "inconclusive":
		sb.WriteString(fmt.Sprintf("  %s\n", yellowText.Render("INCONCLUSIVE")))
	case "ai-assisted":
		sb.WriteString(fmt.Sprintf("  %s\n", redText.Render("AI-ASSISTED")))
	default:
		sb.WriteString(fmt.Sprintf("  %s\n", dimText.Render(r.AISlop.Verdict)))
	}

	if r.AISlop.Confidence > 0 {
		sb.WriteString(fmt.Sprintf("  Confidence: %d%%\n", r.AISlop.Confidence))
	}

	if len(r.AISlop.Evidence) > 0 {
		sb.WriteString("\n")
		for _, ev := range r.AISlop.Evidence {
			sb.WriteString(fmt.Sprintf("  - %s\n", ev))
		}
	}

	if len(r.AISlop.PatternHits) > 0 {
		sb.WriteString(fmt.Sprintf("\n  %s\n", dimText.Render("Pattern matches:")))
		for _, hit := range r.AISlop.PatternHits {
			sb.WriteString(fmt.Sprintf("    %s %s: %s\n",
				dimText.Render("-"),
				dimText.Render(hit.Field),
				dimText.Render(hit.MatchedText),
			))
		}
	}

	sb.WriteString("\n")
	return sb.String()
}

func renderHeader(r *model.PRReport, width int) string {
	var sb strings.Builder

	title := titleStyle.Render(fmt.Sprintf("PR #%d: %s", r.Input.Number, r.Meta.Title))
	author := fmt.Sprintf("Author: %s", blueText.Render("@"+r.Meta.Author))
	age := dimText.Render(formatAge(r.Meta.CreatedAt))
	branch := dimText.Render(fmt.Sprintf("%s -> %s", r.Meta.HeadBranch, r.Meta.BaseBranch))

	draft := ""
	if r.Meta.IsDraft {
		draft = yellowText.Render("  DRAFT")
	}

	status := greenText.Render(r.Meta.State)
	if r.Meta.State == "closed" {
		status = redText.Render("closed")
	}

	ci := r.Meta.CIStatus
	switch ci {
	case "success":
		ci = greenText.Render("passing")
	case "failure":
		ci = redText.Render("failing")
	case "pending":
		ci = yellowText.Render("pending")
	default:
		ci = dimText.Render(ci)
	}

	reviews := dimText.Render(fmt.Sprintf("Reviews: %d", r.Meta.ReviewCount))

	sb.WriteString(title + draft + "\n")
	sb.WriteString(fmt.Sprintf("  %s  |  %s  |  %s\n", author, age, branch))
	sb.WriteString(fmt.Sprintf("  Status: %s  |  %s  |  CI: %s", status, reviews, ci))

	sb.WriteString("\n")
	if r.Author.IsFirstTime {
		sb.WriteString("  " + yellowText.Render("First-time contributor - review with extra care"))
	} else {
		tenure := ""
		if !r.Author.FirstContribDate.IsZero() {
			tenure = fmt.Sprintf(", contributing since %s", r.Author.FirstContribDate.Format("Jan 2006"))
		}
		sb.WriteString(fmt.Sprintf("  %s",
			greenText.Render(fmt.Sprintf("Trusted contributor: %d merged PR(s)%s", r.Author.MergedPRs, tenure)),
		))

		if len(r.Author.TopAreas) > 0 {
			sb.WriteString(fmt.Sprintf("\n  %s %s",
				dimText.Render("Expertise:"),
				strings.Join(r.Author.TopAreas, ", "),
			))

			prDirs := map[string]bool{}
			for _, d := range r.Scope.DirBreakdown {
				prDirs[d.Dir] = true
			}
			overlap := false
			for _, area := range r.Author.TopAreas {
				if prDirs[area] {
					overlap = true
					break
				}
				for dir := range prDirs {
					if strings.HasPrefix(dir, area+"/") || strings.HasPrefix(area, dir+"/") || filepath.Dir(dir) == filepath.Dir(area) {
						overlap = true
						break
					}
				}
			}
			if overlap {
				sb.WriteString("\n  " + greenText.Render("Author has domain expertise in this area"))
			}
		}
	}

	content := sb.String()
	return headerBoxStyle.Width(width - 4).Render(content)
}

func renderScope(r *model.PRReport) string {
	var sb strings.Builder

	sb.WriteString(sectionTitleStyle.Render(" SCOPE"))
	sb.WriteString("\n")

	sb.WriteString(fmt.Sprintf("  Files: %s  |  %s / %s lines  |  %d packages\n",
		boldText.Render(fmt.Sprintf("%d changed", r.Scope.FilesChanged)),
		greenText.Render(fmt.Sprintf("+%d", r.Scope.TotalAdded)),
		redText.Render(fmt.Sprintf("-%d", r.Scope.TotalDeleted)),
		r.Scope.PackagesCount,
	))

	sb.WriteString(fmt.Sprintf("  Type:  %s (%s confidence)\n",
		boldText.Render(string(r.Classify.Type)),
		r.Classify.Confidence,
	))

	complexity := fmt.Sprintf("%d/10", r.Scope.Complexity)
	switch {
	case r.Scope.Complexity >= 7:
		complexity = redText.Render(complexity)
	case r.Scope.Complexity >= 4:
		complexity = yellowText.Render(complexity)
	default:
		complexity = greenText.Render(complexity)
	}
	sb.WriteString(fmt.Sprintf("  Complexity: %s\n", complexity))

	sb.WriteString("\n")
	maxBarWidth := 22
	for _, ds := range r.Scope.DirBreakdown {
		filled := int(ds.Percentage / 100 * float64(maxBarWidth))
		if filled < 1 && ds.LinesAdded > 0 {
			filled = 1
		}
		bar := barFull.Render(strings.Repeat("█", filled)) + barEmpty.Render(strings.Repeat("░", maxBarWidth-filled))
		sb.WriteString(fmt.Sprintf("  %-30s %s  +%-4d (%2.0f%%)\n",
			ds.Dir, bar, ds.LinesAdded, ds.Percentage,
		))
	}

	return sb.String()
}

func renderAISummary(r *model.PRReport) string {
	var sb strings.Builder

	sb.WriteString(sectionTitleStyle.Render(" WHAT THIS PR DOES"))
	sb.WriteString("\n")

	if r.AI == nil || r.AI.Summary == "" {
		sb.WriteString(dimText.Render("  (AI analysis unavailable)"))
		sb.WriteString("\n")
		return sb.String()
	}

	for _, line := range wrapText(r.AI.Summary, 76) {
		sb.WriteString(fmt.Sprintf("  %s\n", line))
	}

	return sb.String()
}

func renderBeforeAfter(r *model.PRReport) string {
	var sb strings.Builder

	sb.WriteString(sectionTitleStyle.Render(" HOW IT WORKED BEFORE vs AFTER"))
	sb.WriteString("\n")

	if r.AI == nil || (r.AI.Before == "" && r.AI.After == "") {
		sb.WriteString(dimText.Render("  (AI analysis unavailable)"))
		sb.WriteString("\n")
		return sb.String()
	}

	sb.WriteString("\n")
	sb.WriteString(dimText.Render("  BEFORE:"))
	sb.WriteString("\n")
	for _, line := range wrapText(r.AI.Before, 74) {
		sb.WriteString(fmt.Sprintf("    %s\n", line))
	}

	sb.WriteString("\n")
	sb.WriteString(boldText.Render("  AFTER:"))
	sb.WriteString("\n")
	for _, line := range wrapText(r.AI.After, 74) {
		sb.WriteString(fmt.Sprintf("    %s\n", greenText.Render(line)))
	}

	return sb.String()
}

func renderAIIssues(r *model.PRReport) string {
	var sb strings.Builder

	sb.WriteString(sectionTitleStyle.Render(" POTENTIAL ISSUES (AI)"))
	sb.WriteString("\n")

	if r.AI == nil || len(r.AI.Issues) == 0 {
		sb.WriteString(greenText.Render("  No issues identified by AI"))
		sb.WriteString("\n")
		return sb.String()
	}

	for _, issue := range r.AI.Issues {
		var icon, label string
		switch issue.Severity {
		case model.SeverityHigh:
			icon = redText.Render("●")
			label = redText.Render("HIGH")
		case model.SeverityMedium:
			icon = yellowText.Render("●")
			label = yellowText.Render("MEDIUM")
		default:
			icon = dimText.Render("●")
			label = dimText.Render("LOW")
		}

		loc := ""
		if issue.File != "" {
			loc = fmt.Sprintf("%s:%d", issue.File, issue.Line)
		}

		sb.WriteString(fmt.Sprintf("\n  %s %s  %s\n", icon, label, dimText.Render(loc)))

		if issue.WhatChanged != "" {
			sb.WriteString(fmt.Sprintf("    %s ", boldText.Render("What changed:")))
			for i, line := range wrapText(issue.WhatChanged, 60) {
				if i == 0 {
					sb.WriteString(fmt.Sprintf("%s\n", line))
				} else {
					sb.WriteString(fmt.Sprintf("                   %s\n", line))
				}
			}
		}

		if issue.WhyRisky != "" {
			sb.WriteString(fmt.Sprintf("    %s ", redText.Render("Risk:")))
			for i, line := range wrapText(issue.WhyRisky, 66) {
				if i == 0 {
					sb.WriteString(fmt.Sprintf("%s\n", line))
				} else {
					sb.WriteString(fmt.Sprintf("            %s\n", line))
				}
			}
		}

		if issue.TradeOff != "" {
			sb.WriteString(fmt.Sprintf("    %s ", yellowText.Render("Trade-off:")))
			for i, line := range wrapText(issue.TradeOff, 62) {
				if i == 0 {
					sb.WriteString(fmt.Sprintf("%s\n", line))
				} else {
					sb.WriteString(fmt.Sprintf("                %s\n", line))
				}
			}
		}

		if issue.Suggestion != "" {
			sb.WriteString(fmt.Sprintf("    %s %s\n", blueText.Render("Fix:"), issue.Suggestion))
		}
	}

	return sb.String()
}

func renderReviewQuestions(r *model.PRReport) string {
	var sb strings.Builder

	sb.WriteString(sectionTitleStyle.Render(" QUESTIONS FOR THE REVIEWER"))
	sb.WriteString("\n")

	if r.AI == nil || len(r.AI.ReviewQuestions) == 0 {
		sb.WriteString(dimText.Render("  (no review questions generated)"))
		sb.WriteString("\n")
		return sb.String()
	}

	for i, q := range r.AI.ReviewQuestions {
		sb.WriteString(fmt.Sprintf("\n  %s %s\n",
			yellowText.Render(fmt.Sprintf("%d.", i+1)),
			boldText.Render(q.Question),
		))
		if q.WhereLook != "" {
			sb.WriteString(fmt.Sprintf("     %s %s\n",
				dimText.Render("Look at:"),
				q.WhereLook,
			))
		}
		if q.HowVerify != "" {
			sb.WriteString(fmt.Sprintf("     %s %s\n",
				dimText.Render("Verify:"),
				q.HowVerify,
			))
		}
	}

	return sb.String()
}

func renderTestAssessment(r *model.PRReport) string {
	var sb strings.Builder

	sb.WriteString(sectionTitleStyle.Render(" TEST COVERAGE"))
	sb.WriteString("\n")

	if r.AI != nil {
		if r.AI.TestVerdict.Sufficient {
			sb.WriteString(fmt.Sprintf("  %s\n", greenText.Render("SUFFICIENT")))
		} else {
			sb.WriteString(fmt.Sprintf("  %s\n", redText.Render("INSUFFICIENT")))
		}
		if r.AI.TestVerdict.Summary != "" {
			for _, line := range wrapText(r.AI.TestVerdict.Summary, 74) {
				sb.WriteString(fmt.Sprintf("  %s\n", dimText.Render(line)))
			}
		}
	}

	if r.AI != nil && len(r.AI.TestVerdict.KeyTestFiles) > 0 {
		sb.WriteString(fmt.Sprintf("\n  %s\n", boldText.Render("Key test files to review:")))
		for _, f := range r.AI.TestVerdict.KeyTestFiles {
			sb.WriteString(fmt.Sprintf("    %s\n", blueText.Render(f)))
		}
	}

	if r.AI != nil && len(r.AI.TestVerdict.CriticalUntested) > 0 {
		sb.WriteString(fmt.Sprintf("\n  %s\n", redText.Render("Critical paths without tests:")))
		for _, cp := range r.AI.TestVerdict.CriticalUntested {
			sb.WriteString(fmt.Sprintf("\n    %s %s\n", redText.Render("-"), boldText.Render(cp.Path)))
			if cp.WhyCritical != "" {
				sb.WriteString(fmt.Sprintf("      %s %s\n", dimText.Render("Why:"), cp.WhyCritical))
			}
			if cp.RegressionRisk != "" {
				sb.WriteString(fmt.Sprintf("      %s %s\n", yellowText.Render("Regression risk:"), cp.RegressionRisk))
			}
		}
	}

	if r.AI != nil && len(r.AI.TestVerdict.MissingScenarios) > 0 {
		sb.WriteString(fmt.Sprintf("\n  %s\n", yellowText.Render("Missing test scenarios:")))
		for _, ms := range r.AI.TestVerdict.MissingScenarios {
			sb.WriteString(fmt.Sprintf("\n    %s %s\n", yellowText.Render("-"), boldText.Render(ms.Scenario)))
			if ms.WhyNeeded != "" {
				sb.WriteString(fmt.Sprintf("      %s %s\n", dimText.Render("Risk:"), ms.WhyNeeded))
			}
		}
	}

	return sb.String()
}

func renderRisks(r *model.PRReport) string {
	var sb strings.Builder

	sb.WriteString(sectionTitleStyle.Render(" RISK FLAGS (heuristic + AI)"))
	sb.WriteString("\n")

	grouped := map[model.Severity][]model.RiskFlag{}
	for _, rf := range r.Risks {
		grouped[rf.Severity] = append(grouped[rf.Severity], rf)
	}

	significantCount := len(grouped[model.SeverityHigh]) + len(grouped[model.SeverityMedium])
	if significantCount == 0 {
		sb.WriteString(greenText.Render("  No significant risk flags detected"))
		sb.WriteString("\n")
		return sb.String()
	}

	commentaryMap := map[string]model.RiskComment{}
	if r.AI != nil {
		for _, rc := range r.AI.RiskCommentary {
			key := fmt.Sprintf("%s:%d", rc.File, rc.Line)
			commentaryMap[key] = rc
		}
	}

	for _, sev := range []model.Severity{model.SeverityHigh, model.SeverityMedium} {
		flags := grouped[sev]
		if len(flags) == 0 {
			continue
		}

		var icon, label string
		switch sev {
		case model.SeverityHigh:
			icon = redText.Render("●")
			label = redText.Render("HIGH")
		case model.SeverityMedium:
			icon = yellowText.Render("●")
			label = yellowText.Render("MEDIUM")
		}

		sb.WriteString(fmt.Sprintf("\n  %s %s\n", icon, label))
		for _, f := range flags {
			loc := fmt.Sprintf("%s:%d", f.File, f.Line)
			sb.WriteString(fmt.Sprintf("  ├─ %-30s %s\n", dimText.Render(loc), f.Description))
			if f.Code != "" {
				sb.WriteString(fmt.Sprintf("  │  %s\n", dimText.Render(f.Code)))
			}

			key := fmt.Sprintf("%s:%d", f.File, f.Line)
			if rc, ok := commentaryMap[key]; ok {
				if rc.IsRealProblem {
					sb.WriteString(fmt.Sprintf("  │  %s %s\n",
						redText.Render("AI:"),
						rc.AIAssessment,
					))
				} else {
					sb.WriteString(fmt.Sprintf("  │  %s %s\n",
						dimText.Render("AI:"),
						dimText.Render(rc.AIAssessment),
					))
				}
			}
		}
	}

	return sb.String()
}

func renderReviewers(r *model.PRReport) string {
	var sb strings.Builder

	sb.WriteString(sectionTitleStyle.Render(" SUGGESTED REVIEWERS"))
	sb.WriteString("\n")

	if len(r.Reviewers) == 0 {
		sb.WriteString(dimText.Render("  No reviewer suggestions (insufficient blame data)"))
		sb.WriteString("\n")
		return sb.String()
	}

	for _, rev := range r.Reviewers {
		confStyle := dimText
		switch rev.Confidence {
		case "high":
			confStyle = greenText
		case "medium":
			confStyle = yellowText
		}

		sb.WriteString(fmt.Sprintf("\n  %s (confidence: %s)\n",
			blueText.Render("@"+rev.Login),
			confStyle.Render(rev.Confidence),
		))
		sb.WriteString(fmt.Sprintf("  ├─ %s\n", rev.Reason))
		sb.WriteString(fmt.Sprintf("  ├─ Last active: %s\n", dimText.Render(rev.LastActive)))
		sb.WriteString(fmt.Sprintf("  └─ Review: %s\n", dimText.Render(strings.Join(rev.Files, ", "))))
	}

	return sb.String()
}

func renderPersonaReview(r *model.PRReport) string {
	if r.PersonaReview == "" {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(sectionTitleStyle.Render(" AI PR REVIEW (informed by domain experts)"))
	sb.WriteString("\n\n")

	for _, line := range strings.Split(r.PersonaReview, "\n") {
		if strings.TrimSpace(line) == "" {
			sb.WriteString("\n")
			continue
		}
		for _, wrapped := range wrapText(line, 76) {
			sb.WriteString(fmt.Sprintf("  %s\n", wrapped))
		}
	}

	return sb.String()
}

func renderVerdict(r *model.PRReport) string {
	var sb strings.Builder

	sb.WriteString(sectionTitleStyle.Render(" VERDICT"))
	sb.WriteString("\n")

	var verdictStyle func(strs ...string) string
	switch r.Verdict {
	case model.VerdictApprove:
		verdictStyle = greenText.Render
	case model.VerdictRequestChanges:
		verdictStyle = redText.Render
	case model.VerdictDiscuss:
		verdictStyle = yellowText.Render
	}

	sb.WriteString(fmt.Sprintf("  Recommend: %s\n", verdictStyle(string(r.Verdict))))
	if r.VerdictNote != "" {
		sb.WriteString("\n")
		for _, line := range wrapText(r.VerdictNote, 74) {
			sb.WriteString(fmt.Sprintf("  %s\n", dimText.Render(line)))
		}
	}

	return sb.String()
}

func formatAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	default:
		return fmt.Sprintf("%d weeks ago", int(d.Hours()/(24*7)))
	}
}

func wrapText(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{text}
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var lines []string
	current := words[0]

	for _, word := range words[1:] {
		if len(current)+1+len(word) > maxWidth {
			lines = append(lines, current)
			current = word
		} else {
			current += " " + word
		}
	}
	lines = append(lines, current)

	return lines
}
