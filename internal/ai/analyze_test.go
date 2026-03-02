package ai

import "testing"

func TestParseResponse_JSONFence(t *testing.T) {
	t.Parallel()

	raw := "```json\n" +
		"{\n" +
		"  \"summary\": \"Summary text\",\n" +
		"  \"before_after\": {\"before\": \"before\", \"after\": \"after\"},\n" +
		"  \"potential_issues\": [\n" +
		"    {\n" +
		"      \"severity\": \"high\",\n" +
		"      \"file\": \"x.go\",\n" +
		"      \"line\": 10,\n" +
		"      \"what_changed\": \"changed\",\n" +
		"      \"why_risky\": \"risky\",\n" +
		"      \"trade_off\": \"tradeoff\",\n" +
		"      \"suggestion\": \"fix it\"\n" +
		"    }\n" +
		"  ],\n" +
		"  \"review_questions\": [\n" +
		"    {\n" +
		"      \"question\": \"question?\",\n" +
		"      \"where_to_look\": \"x.go:10\",\n" +
		"      \"how_to_verify\": \"go test ./...\"\n" +
		"    }\n" +
		"  ],\n" +
		"  \"test_verdict\": {\n" +
		"    \"sufficient\": false,\n" +
		"    \"summary\": \"need tests\",\n" +
		"    \"critical_untested\": [\n" +
		"      {\n" +
		"        \"path\": \"critical/path\",\n" +
		"        \"why_critical\": \"important\",\n" +
		"        \"regression_risk\": \"could regress\"\n" +
		"      }\n" +
		"    ],\n" +
		"    \"key_test_files\": [\"x_test.go\"],\n" +
		"    \"missing_scenarios\": [\n" +
		"      {\n" +
		"        \"scenario\": \"error path\",\n" +
		"        \"why_needed\": \"runtime failure\"\n" +
		"      }\n" +
		"    ]\n" +
		"  },\n" +
		"  \"risk_commentary\": [\n" +
		"    {\n" +
		"      \"file\": \"x.go\",\n" +
		"      \"line\": 11,\n" +
		"      \"pattern\": \"ignored err\",\n" +
		"      \"ai_assessment\": \"real\",\n" +
		"      \"is_real_problem\": true\n" +
		"    }\n" +
		"  ],\n" +
		"  \"verdict\": \"request_changes\",\n" +
		"  \"verdict_reason\": \"high risk\"\n" +
		"}\n" +
		"```"

	analysis, err := parseResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if analysis == nil {
		t.Fatalf("expected analysis, got nil")
	}
	if analysis.Summary != "Summary text" {
		t.Fatalf("unexpected summary: %q", analysis.Summary)
	}
	if analysis.Verdict != "request_changes" {
		t.Fatalf("unexpected verdict: %q", analysis.Verdict)
	}
	if len(analysis.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(analysis.Issues))
	}
}
