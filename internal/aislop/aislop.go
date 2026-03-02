package aislop

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/drpaneas/prview/internal/model"
)

type rawResult struct {
	Verdict     string `json:"verdict"`
	PatternHits []struct {
		Field       string `json:"field"`
		Pattern     string `json:"pattern"`
		MatchedText string `json:"matched_text"`
	} `json:"pattern-hits"`
	LLMVerdict *struct {
		Verdict    string   `json:"verdict"`
		Confidence int      `json:"confidence"`
		Evidence   []string `json:"evidence"`
	} `json:"llm_verdict"`
}

func findBinary() string {
	if p, err := exec.LookPath("aislop"); err == nil {
		return p
	}
	return ""
}

func Detect(ctx context.Context, prURL string) (*model.AISlopResult, error) {
	bin := findBinary()
	if bin == "" {
		return nil, fmt.Errorf("aislop not found on PATH - install it: https://github.com/drpaneas/aislop")
	}

	args := []string{prURL, "-f", "json", "--llm", "--llm-provider", "claude"}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = os.Environ()

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			output = exitErr.Stderr
		}
		return nil, fmt.Errorf("aislop failed: %w (output: %s)", err, string(output))
	}

	var raw rawResult
	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("parsing aislop output: %w", err)
	}

	result := &model.AISlopResult{
		Verdict: raw.Verdict,
	}

	for _, hit := range raw.PatternHits {
		result.PatternHits = append(result.PatternHits, model.PatternHit{
			Field:       hit.Field,
			Pattern:     hit.Pattern,
			MatchedText: hit.MatchedText,
		})
	}

	if raw.LLMVerdict != nil {
		result.LLMVerdict = raw.LLMVerdict.Verdict
		result.Confidence = raw.LLMVerdict.Confidence
		result.Evidence = raw.LLMVerdict.Evidence
	}

	return result, nil
}
