package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/drpaneas/prview/internal/model"
)

const systemPrompt = `You are an expert code reviewer helping open-source maintainers analyze pull requests.
You receive PR metadata, diffs, and heuristic risk flags. Your job is to provide a clear, actionable analysis.

Respond ONLY with valid JSON matching this exact schema:
{
  "summary": "2-3 sentence plain-language explanation of what this PR does and why",
  "before_after": {
    "before": "How the code/system worked before this PR (conceptual, not line-by-line)",
    "after": "How it works after this PR (what changed architecturally or behaviorally)"
  },
  "potential_issues": [
    {
      "severity": "high|medium|low",
      "file": "path/to/file.go",
      "line": 42,
      "what_changed": "What specifically was changed at this location",
      "why_risky": "Why this is concerning - describe the worst-case scenario if this goes wrong",
      "trade_off": "What benefit this change provides vs what risk it introduces",
      "suggestion": "Concrete fix or verification step"
    }
  ],
  "review_questions": [
    {
      "question": "The question the reviewer needs to answer",
      "where_to_look": "Specific file paths, line ranges, or code locations to inspect",
      "how_to_verify": "Commands to run, things to grep for, or manual steps to verify"
    }
  ],
  "test_verdict": {
    "sufficient": true,
    "summary": "One sentence: is testing sufficient and why",
    "critical_untested": [
      {
        "path": "the critical code path that lacks tests",
        "why_critical": "why this path MUST be tested - what breaks if it has a bug",
        "regression_risk": "was this previously tested and now the test is gone? does this risk breaking existing behavior?"
      }
    ],
    "key_test_files": ["test files the reviewer should examine"],
    "missing_scenarios": [
      {
        "scenario": "the missing test scenario",
        "why_needed": "what could go wrong in production without this test"
      }
    ]
  },
  "risk_commentary": [
    {
      "file": "path/to/file.go",
      "line": 42,
      "pattern": "what the heuristic flagged",
      "ai_assessment": "Is this a real problem in this context? Brief explanation.",
      "is_real_problem": true
    }
  ],
  "verdict": "approve|request_changes|discuss",
  "verdict_reason": "1-2 sentence justification for the verdict"
}

Rules:
- Be specific. Reference actual file names and line numbers from the diff.
- Focus on things that matter: bugs, security, correctness, missing edge cases.
- Do NOT just restate what the diff shows. Explain WHY something is a concern.
- Keep the summary understandable by someone who hasn't read the code.
- For before/after, explain the conceptual change, not a line-by-line walkthrough.
- For potential_issues:
  - NEVER say something was "simplified" without explaining HOW it was simplified.
  - NEVER say "verify X" without explaining what breaks if you do not.
  - Always describe the worst-case scenario in why_risky.
  - Always explain the trade-off: what benefit vs what risk.
  - Quality over quantity - only flag real concerns, not style nits.
- For review_questions:
  - Always include where_to_look with specific file paths or line ranges.
  - Always include how_to_verify with concrete steps: commands to run, things to grep for, specific assertions to check.
- For test_verdict:
  - Be pragmatic. Only flag missing tests for critical paths - happy path and primary error path.
  - Do NOT demand tests for trivial getters, constructors, or boilerplate.
  - Point to the specific test files the reviewer should look at.
  - For each critical_untested path, explain the regression risk: could this break something that was working before?
  - For each missing_scenario, explain what production failure it could cause.
- For risk_commentary:
  - You will receive a list of heuristic risk flags. For each one, assess whether it is a REAL problem in the context of this specific PR or just noise from the pattern matcher.
  - Be honest: if a defer Close() is fine because the resource is managed elsewhere, say so.
  - If the heuristic found a real issue, confirm it and explain why it matters here.`

func AnalyzePR(ctx context.Context, client Caller, data *model.PRData, risks []model.RiskFlag) (*model.AIAnalysis, error) {
	prompt := buildPrompt(data, risks)

	raw, err := client.Call(ctx, systemPrompt, prompt)
	if err != nil {
		return nil, fmt.Errorf("AI analysis failed: %w", err)
	}

	return parseResponse(raw)
}

func buildPrompt(data *model.PRData, risks []model.RiskFlag) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# PR #%d: %s\n", data.Input.Number, data.Meta.Title))
	sb.WriteString(fmt.Sprintf("Author: %s\n", data.Meta.Author))
	sb.WriteString(fmt.Sprintf("Branch: %s -> %s\n", data.Meta.HeadBranch, data.Meta.BaseBranch))
	sb.WriteString(fmt.Sprintf("State: %s | CI: %s | Reviews: %d\n\n", data.Meta.State, data.Meta.CIStatus, data.Meta.ReviewCount))

	sb.WriteString("## Files Changed\n\n")
	for _, d := range data.Diffs {
		sb.WriteString(fmt.Sprintf("- %s (%s, +%d/-%d)\n", d.Path, d.Status, d.Additions, d.Deletions))
	}

	sb.WriteString("\n## Diffs\n\n")
	totalPatchLen := 0
	const maxPatchBytes = 80000
	for _, d := range data.Diffs {
		if d.Patch == "" {
			continue
		}
		patch := d.Patch
		if totalPatchLen+len(patch) > maxPatchBytes {
			remaining := maxPatchBytes - totalPatchLen
			if remaining > 200 {
				patch = patch[:remaining] + "\n... (truncated)"
			} else {
				sb.WriteString(fmt.Sprintf("\n### %s\n(diff truncated - PR too large)\n", d.Path))
				continue
			}
		}
		totalPatchLen += len(patch)
		sb.WriteString(fmt.Sprintf("\n### %s (%s)\n```\n%s\n```\n", d.Path, d.Status, patch))
	}

	if len(risks) > 0 {
		sb.WriteString("\n## Heuristic Risk Flags (please assess each one)\n\n")
		for _, r := range risks {
			if r.Severity == model.SeverityLow {
				continue
			}
			sb.WriteString(fmt.Sprintf("- [%s] %s:%d - %s", r.Severity, r.File, r.Line, r.Description))
			if r.Code != "" {
				sb.WriteString(fmt.Sprintf(" | code: `%s`", r.Code))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

type aiResponse struct {
	Summary     string `json:"summary"`
	BeforeAfter struct {
		Before string `json:"before"`
		After  string `json:"after"`
	} `json:"before_after"`
	PotentialIssues []struct {
		Severity    string `json:"severity"`
		File        string `json:"file"`
		Line        int    `json:"line"`
		WhatChanged string `json:"what_changed"`
		WhyRisky    string `json:"why_risky"`
		TradeOff    string `json:"trade_off"`
		Suggestion  string `json:"suggestion"`
	} `json:"potential_issues"`
	ReviewQuestions []struct {
		Question  string `json:"question"`
		WhereLook string `json:"where_to_look"`
		HowVerify string `json:"how_to_verify"`
	} `json:"review_questions"`
	TestVerdict struct {
		Sufficient       bool   `json:"sufficient"`
		Summary          string `json:"summary"`
		CriticalUntested []struct {
			Path           string `json:"path"`
			WhyCritical    string `json:"why_critical"`
			RegressionRisk string `json:"regression_risk"`
		} `json:"critical_untested"`
		KeyTestFiles     []string `json:"key_test_files"`
		MissingScenarios []struct {
			Scenario  string `json:"scenario"`
			WhyNeeded string `json:"why_needed"`
		} `json:"missing_scenarios"`
	} `json:"test_verdict"`
	RiskCommentary []struct {
		File          string `json:"file"`
		Line          int    `json:"line"`
		Pattern       string `json:"pattern"`
		AIAssessment  string `json:"ai_assessment"`
		IsRealProblem bool   `json:"is_real_problem"`
	} `json:"risk_commentary"`
	Verdict       string `json:"verdict"`
	VerdictReason string `json:"verdict_reason"`
}

func parseResponse(raw string) (*model.AIAnalysis, error) {
	cleaned := raw
	if idx := strings.Index(cleaned, "```json"); idx >= 0 {
		cleaned = cleaned[idx+7:]
	} else if idx := strings.Index(cleaned, "```"); idx >= 0 {
		cleaned = cleaned[idx+3:]
	}
	if idx := strings.LastIndex(cleaned, "```"); idx >= 0 {
		cleaned = cleaned[:idx]
	}
	cleaned = strings.TrimSpace(cleaned)

	var resp aiResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		return nil, fmt.Errorf("parsing AI response: %w\nRaw response:\n%s", err, raw[:min(len(raw), 500)])
	}

	tv := model.TestVerdict{
		Sufficient:   resp.TestVerdict.Sufficient,
		Summary:      resp.TestVerdict.Summary,
		KeyTestFiles: resp.TestVerdict.KeyTestFiles,
	}
	for _, cu := range resp.TestVerdict.CriticalUntested {
		tv.CriticalUntested = append(tv.CriticalUntested, model.CriticalPath{
			Path:           cu.Path,
			WhyCritical:    cu.WhyCritical,
			RegressionRisk: cu.RegressionRisk,
		})
	}
	for _, ms := range resp.TestVerdict.MissingScenarios {
		tv.MissingScenarios = append(tv.MissingScenarios, model.MissingScenario{
			Scenario:  ms.Scenario,
			WhyNeeded: ms.WhyNeeded,
		})
	}

	analysis := &model.AIAnalysis{
		Summary:       resp.Summary,
		Before:        resp.BeforeAfter.Before,
		After:         resp.BeforeAfter.After,
		Verdict:       resp.Verdict,
		VerdictReason: resp.VerdictReason,
		TestVerdict:   tv,
	}

	for _, issue := range resp.PotentialIssues {
		sev := model.SeverityLow
		switch issue.Severity {
		case "high":
			sev = model.SeverityHigh
		case "medium":
			sev = model.SeverityMedium
		}
		analysis.Issues = append(analysis.Issues, model.AIIssue{
			Severity:    sev,
			File:        issue.File,
			Line:        issue.Line,
			WhatChanged: issue.WhatChanged,
			WhyRisky:    issue.WhyRisky,
			TradeOff:    issue.TradeOff,
			Suggestion:  issue.Suggestion,
		})
	}

	for _, q := range resp.ReviewQuestions {
		analysis.ReviewQuestions = append(analysis.ReviewQuestions, model.ReviewQuestion{
			Question:  q.Question,
			WhereLook: q.WhereLook,
			HowVerify: q.HowVerify,
		})
	}

	for _, rc := range resp.RiskCommentary {
		analysis.RiskCommentary = append(analysis.RiskCommentary, model.RiskComment{
			File:          rc.File,
			Line:          rc.Line,
			Pattern:       rc.Pattern,
			AIAssessment:  rc.AIAssessment,
			IsRealProblem: rc.IsRealProblem,
		})
	}

	return analysis, nil
}
