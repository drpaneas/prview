package persona

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/drpaneas/prview/internal/ai"
	"github.com/drpaneas/prview/internal/model"
)

func ReadSkills(dir, username string) (string, error) {
	lower := strings.ToLower(username)
	var sb strings.Builder
	skillTypes := []string{"coding-style", "code-reviewer", "developer-profile"}

	for _, skillType := range skillTypes {
		path := filepath.Join(dir, lower+"-"+skillType, "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("=== %s: %s ===\n", username, skillType))
		sb.Write(data)
		sb.WriteString("\n\n")
	}

	if sb.Len() == 0 {
		return "", fmt.Errorf("no skill files found for %s (looked in %s/%s-*/SKILL.md)", username, dir, lower)
	}
	return sb.String(), nil
}

const personaPrompt = `You are writing a thorough PR review that synthesizes the perspectives of multiple experienced reviewers whose personas are provided below.

Your task: produce a SINGLE unified review - NOT separate sections per reviewer. Blend their collective expertise, priorities, and instincts into one comprehensive review. Do NOT mention any reviewer by name or username. The output should read as one cohesive expert review that happens to reflect the combined knowledge of all these people.

The review must cover these areas in depth, drawing from each reviewer's documented priorities:

1. ARCHITECTURE & DESIGN: Evaluate the structural changes. Is the before-to-after transition sound? Are there propagation risks across the codebase? Does this maintain design intent? Comment on any new patterns, removed abstractions, or shifted responsibilities.

2. CORRECTNESS & SAFETY: Check for nil safety, error handling gaps, Kubernetes API pattern compliance, proper use of contexts. Trace how changes propagate through controllers, services, and tests. Flag any path where a change could cause a runtime failure.

3. TEST COVERAGE ASSESSMENT: Are the critical paths tested? Is the test refactoring adequate? Are there scenarios that were previously covered and are now missing? Be pragmatic - only flag tests that actually matter for production safety.

4. OPERATIONAL CONCERNS: What happens during restarts, under rate limits, at scale? Are metrics preserved across controller lifecycles? Are there edge cases around timing, ordering, or state loss?

5. SPECIFIC CONCERNS: Reference exact files, functions, and line numbers from the PR analysis. For each concern, explain what could go wrong concretely - not vague "this might be an issue" statements.

6. WHAT LOOKS GOOD: Acknowledge what the PR does well. Simplified test setup? Cleaner abstractions? Better separation of concerns? Say so.

7. VERDICT: Should this be approved, should changes be requested, or does it need discussion? Be specific about what needs to happen before merge.

Rules:
- Write a FULL review - aim for thorough coverage, not brevity.
- Reference SPECIFIC files, functions, and line numbers from the PR analysis provided.
- Do NOT invent concerns that aren't supported by the PR analysis data.
- Do NOT mention any reviewer names or usernames.
- Do NOT use section headers with "@username".
- Write in a direct, technical tone - no fluff, no hedging.
- Respond in plain text, not JSON.`

func buildReviewContext(report *model.PRReport) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# PR #%d: %s\n", report.Input.Number, report.Meta.Title))
	sb.WriteString(fmt.Sprintf("Author: @%s\n", report.Meta.Author))
	sb.WriteString(fmt.Sprintf("Branch: %s -> %s\n", report.Meta.HeadBranch, report.Meta.BaseBranch))
	sb.WriteString(fmt.Sprintf("Files changed: %d  |  +%d / -%d lines\n\n",
		report.Scope.FilesChanged, report.Scope.TotalAdded, report.Scope.TotalDeleted))

	if report.AI != nil {
		sb.WriteString("## What this PR does\n")
		sb.WriteString(report.AI.Summary + "\n\n")

		sb.WriteString("## Before vs After\n")
		sb.WriteString("BEFORE: " + report.AI.Before + "\n")
		sb.WriteString("AFTER: " + report.AI.After + "\n\n")

		if len(report.AI.Issues) > 0 {
			sb.WriteString("## Issues found by analysis\n")
			for _, issue := range report.AI.Issues {
				sb.WriteString(fmt.Sprintf("- [%s] %s:%d - %s\n", issue.Severity, issue.File, issue.Line, issue.WhatChanged))
				if issue.WhyRisky != "" {
					sb.WriteString(fmt.Sprintf("  Risk: %s\n", issue.WhyRisky))
				}
				if issue.TradeOff != "" {
					sb.WriteString(fmt.Sprintf("  Trade-off: %s\n", issue.TradeOff))
				}
			}
			sb.WriteString("\n")
		}

		sb.WriteString("## Test verdict\n")
		if report.AI.TestVerdict.Sufficient {
			sb.WriteString("SUFFICIENT: ")
		} else {
			sb.WriteString("INSUFFICIENT: ")
		}
		sb.WriteString(report.AI.TestVerdict.Summary + "\n")
		for _, cp := range report.AI.TestVerdict.CriticalUntested {
			sb.WriteString(fmt.Sprintf("- Critical untested: %s (%s)\n", cp.Path, cp.WhyCritical))
		}
		for _, ms := range report.AI.TestVerdict.MissingScenarios {
			sb.WriteString(fmt.Sprintf("- Missing scenario: %s (%s)\n", ms.Scenario, ms.WhyNeeded))
		}
		sb.WriteString("\n")

		if len(report.AI.RiskCommentary) > 0 {
			sb.WriteString("## Heuristic risk flags with AI assessment\n")
			for _, rc := range report.AI.RiskCommentary {
				real := "noise"
				if rc.IsRealProblem {
					real = "REAL"
				}
				sb.WriteString(fmt.Sprintf("- [%s] %s:%d - %s: %s\n", real, rc.File, rc.Line, rc.Pattern, rc.AIAssessment))
			}
			sb.WriteString("\n")
		}

		sb.WriteString(fmt.Sprintf("## Overall verdict: %s\n", report.AI.VerdictReason))
	}

	sb.WriteString("\n## Changed files\n")
	for _, d := range report.Scope.DirBreakdown {
		sb.WriteString(fmt.Sprintf("- %s (+%d lines, %.0f%%)\n", d.Dir, d.LinesAdded, d.Percentage))
	}

	return sb.String()
}

func GeneratePersonaReview(ctx context.Context, client *ai.Client, skills string, report *model.PRReport) (string, error) {
	var sb strings.Builder
	sb.WriteString(buildReviewContext(report))
	sb.WriteString("\n\n# Reviewer Personas\n\n")
	sb.WriteString(skills)

	result, err := client.Call(ctx, personaPrompt, sb.String())
	if err != nil {
		return "", fmt.Errorf("generating persona review: %w", err)
	}
	return result, nil
}

func GenerateReview(ctx context.Context, client *ai.Client, personaDir string, reviewers []model.Reviewer, report *model.PRReport) (string, error) {
	if len(reviewers) == 0 {
		return "", nil
	}

	if personaDir == "" {
		return "", fmt.Errorf("--persona-dir is required for persona-based review")
	}

	var allSkills strings.Builder
	var processedNames []string
	var missingNames []string

	for _, rev := range reviewers {
		skills, err := ReadSkills(personaDir, rev.Login)
		if err != nil {
			missingNames = append(missingNames, rev.Login)
			continue
		}
		allSkills.WriteString(skills)
		processedNames = append(processedNames, rev.Login)
	}

	if len(missingNames) > 0 {
		fmt.Fprintf(os.Stderr, "  persona: no pre-crawled data for: %s\n", strings.Join(missingNames, ", "))
		fmt.Fprintf(os.Stderr, "  persona: run 'devlica <username>' to generate their persona\n")
	}

	if len(processedNames) == 0 {
		return "", fmt.Errorf("no pre-crawled personas found for any reviewer in %s", personaDir)
	}

	return GeneratePersonaReview(ctx, client, allSkills.String(), report)
}
