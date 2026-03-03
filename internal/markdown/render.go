package markdown

import (
	"fmt"
	"strings"
	"time"

	"github.com/drpaneas/prview/internal/model"
)

func Render(r *model.PRReport) string {
	var sb strings.Builder

	sb.WriteString(renderHeader(r))
	sb.WriteString(renderAISlop(r))
	sb.WriteString(renderScope(r))
	sb.WriteString(renderAISummary(r))
	sb.WriteString(renderBeforeAfter(r))
	sb.WriteString(renderAIIssues(r))
	sb.WriteString(renderReviewQuestions(r))
	sb.WriteString(renderRisks(r))
	sb.WriteString(renderReviewers(r))
	sb.WriteString(renderVerdict(r))

	return sb.String()
}

func renderHeader(r *model.PRReport) string {
	var sb strings.Builder

	draft := ""
	if r.Meta.IsDraft {
		draft = " `DRAFT`"
	}

	sb.WriteString(fmt.Sprintf("## PR #%d: %s%s\n\n", r.Input.Number, r.Meta.Title, draft))

	sb.WriteString(fmt.Sprintf("**Author:** @%s | **Age:** %s | **Branch:** `%s` -> `%s`\n\n",
		r.Meta.Author, formatAge(r.Meta.CreatedAt),
		r.Meta.HeadBranch, r.Meta.BaseBranch))

	ci := r.Meta.CIStatus
	switch ci {
	case "success":
		ci = "passing"
	case "failure":
		ci = "failing"
	}

	sb.WriteString(fmt.Sprintf("**Status:** %s | **Reviews:** %d | **CI:** %s\n\n",
		r.Meta.State, r.Meta.ReviewCount, ci))

	if r.Author.IsFirstTime {
		sb.WriteString("> :warning: **First-time contributor** - review with extra care\n\n")
	} else if r.Author.MergedPRs > 0 {
		tenure := ""
		if !r.Author.FirstContribDate.IsZero() {
			tenure = fmt.Sprintf(", contributing since %s", r.Author.FirstContribDate.Format("Jan 2006"))
		}
		sb.WriteString(fmt.Sprintf("> Trusted contributor: %d merged PR(s)%s\n\n", r.Author.MergedPRs, tenure))
	}

	return sb.String()
}

func renderAISlop(r *model.PRReport) string {
	if r.AISlop == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("### AI-Generated PR Check\n\n")

	icon := ""
	switch r.AISlop.Verdict {
	case "human":
		icon = ":white_check_mark: **HUMAN-WRITTEN**"
	case "inconclusive":
		icon = ":grey_question: **INCONCLUSIVE**"
	case "ai-assisted":
		icon = ":robot: **AI-ASSISTED**"
	default:
		icon = r.AISlop.Verdict
	}

	sb.WriteString(icon)
	if r.AISlop.Confidence > 0 {
		sb.WriteString(fmt.Sprintf(" (confidence: %d%%)", r.AISlop.Confidence))
	}
	sb.WriteString("\n\n")

	if len(r.AISlop.Evidence) > 0 {
		for _, ev := range r.AISlop.Evidence {
			sb.WriteString(fmt.Sprintf("- %s\n", ev))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func renderScope(r *model.PRReport) string {
	var sb strings.Builder

	sb.WriteString("### Scope\n\n")

	sb.WriteString(fmt.Sprintf("**%d files changed** | +%d / -%d lines | %d packages\n\n",
		r.Scope.FilesChanged,
		r.Scope.TotalAdded, r.Scope.TotalDeleted,
		r.Scope.PackagesCount))

	sb.WriteString(fmt.Sprintf("**Type:** %s (%s confidence) | **Complexity:** %d/10\n\n",
		r.Classify.Type, r.Classify.Confidence, r.Scope.Complexity))

	return sb.String()
}

func renderAISummary(r *model.PRReport) string {
	if r.AI == nil || r.AI.Summary == "" {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("### What This PR Does\n\n")
	sb.WriteString(r.AI.Summary + "\n\n")
	return sb.String()
}

func renderBeforeAfter(r *model.PRReport) string {
	if r.AI == nil || (r.AI.Before == "" && r.AI.After == "") {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("### Before vs After\n\n")
	sb.WriteString(fmt.Sprintf("**Before:** %s\n\n", r.AI.Before))
	sb.WriteString(fmt.Sprintf("**After:** %s\n\n", r.AI.After))
	return sb.String()
}

func renderAIIssues(r *model.PRReport) string {
	if r.AI == nil {
		return ""
	}

	var high []model.AIIssue
	for _, issue := range r.AI.Issues {
		if issue.Severity == model.SeverityHigh {
			high = append(high, issue)
		}
	}
	if len(high) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("### Potential Issues\n\n")

	for _, issue := range high {
		loc := ""
		if issue.File != "" {
			loc = fmt.Sprintf(" `%s:%d`", issue.File, issue.Line)
		}

		sb.WriteString(fmt.Sprintf(":red_circle: **%s**%s\n\n", issue.Severity, loc))

		if issue.WhatChanged != "" {
			sb.WriteString(fmt.Sprintf("**What changed:** %s\n\n", issue.WhatChanged))
		}
		if issue.WhyRisky != "" {
			sb.WriteString(fmt.Sprintf("**Risk:** %s\n\n", issue.WhyRisky))
		}
		if issue.TradeOff != "" {
			sb.WriteString(fmt.Sprintf("**Trade-off:** %s\n\n", issue.TradeOff))
		}
		if issue.Suggestion != "" {
			sb.WriteString(fmt.Sprintf("**Fix:** %s\n\n", issue.Suggestion))
		}

		sb.WriteString("---\n\n")
	}

	return sb.String()
}

func renderReviewQuestions(r *model.PRReport) string {
	if r.AI == nil || len(r.AI.ReviewQuestions) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("### Questions for the Reviewer\n\n")

	for i, q := range r.AI.ReviewQuestions {
		sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, q.Question))
		if q.WhereLook != "" {
			sb.WriteString(fmt.Sprintf("   - Look at: %s\n", q.WhereLook))
		}
		if q.HowVerify != "" {
			sb.WriteString(fmt.Sprintf("   - Verify: %s\n", q.HowVerify))
		}
	}
	sb.WriteString("\n")

	return sb.String()
}

func renderRisks(r *model.PRReport) string {
	grouped := map[model.Severity][]model.RiskFlag{}
	for _, rf := range r.Risks {
		grouped[rf.Severity] = append(grouped[rf.Severity], rf)
	}

	significantCount := len(grouped[model.SeverityHigh]) + len(grouped[model.SeverityMedium])
	if significantCount == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("### Risk Flags\n\n")

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

		icon := ":yellow_circle:"
		if sev == model.SeverityHigh {
			icon = ":red_circle:"
		}

		sb.WriteString(fmt.Sprintf("%s **%s**\n\n", icon, sev))
		for _, f := range flags {
			sb.WriteString(fmt.Sprintf("- `%s:%d` - %s", f.File, f.Line, f.Description))
			if f.Code != "" {
				sb.WriteString(fmt.Sprintf(" (`%s`)", f.Code))
			}

			key := fmt.Sprintf("%s:%d", f.File, f.Line)
			if rc, ok := commentaryMap[key]; ok {
				label := "noise"
				if rc.IsRealProblem {
					label = "real"
				}
				sb.WriteString(fmt.Sprintf("\n  - AI [%s]: %s", label, rc.AIAssessment))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func renderReviewers(r *model.PRReport) string {
	if len(r.Reviewers) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("### Suggested Reviewers\n\n")

	for _, rev := range r.Reviewers {
		sb.WriteString(fmt.Sprintf("- **@%s** (%s confidence) - %s | Last active: %s\n",
			rev.Login, rev.Confidence, rev.Reason, rev.LastActive))
	}
	sb.WriteString("\n")

	return sb.String()
}

func renderVerdict(r *model.PRReport) string {
	var sb strings.Builder

	sb.WriteString("### Verdict\n\n")

	icon := ":yellow_circle:"
	switch r.Verdict {
	case model.VerdictApprove:
		icon = ":white_check_mark:"
	case model.VerdictRequestChanges:
		icon = ":x:"
	}

	sb.WriteString(fmt.Sprintf("%s **%s**\n\n", icon, r.Verdict))

	if r.VerdictNote != "" {
		sb.WriteString(r.VerdictNote + "\n\n")
	}

	sb.WriteString("---\n*Generated by [prview](https://github.com/drpaneas/prview)*\n")

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
