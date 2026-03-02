package analyzer

import (
	"path/filepath"
	"strings"

	"github.com/drpaneas/prview/internal/model"
)

func ClassifyChange(diffs []model.FileDiff, meta model.PRMetadata) model.ClassifyResult {
	counts := struct{ test, docs, deps, config, impl int }{}

	for _, d := range diffs {
		base := filepath.Base(d.Path)
		dir := filepath.Dir(d.Path)
		ext := filepath.Ext(d.Path)

		switch {
		case strings.HasSuffix(base, "_test.go"):
			counts.test++
		case base == "go.mod" || base == "go.sum" || base == "package.json" || base == "package-lock.json":
			counts.deps++
		case ext == ".md" || strings.HasPrefix(dir, "docs"):
			counts.docs++
		case base == ".gitignore" || ext == ".yaml" || ext == ".yml" || ext == ".toml" || ext == ".json":
			counts.config++
		default:
			counts.impl++
		}
	}

	total := len(diffs)
	if total == 0 {
		return model.ClassifyResult{Type: model.ChangeTypeMixed, Confidence: "low"}
	}

	if counts.test == total {
		return model.ClassifyResult{Type: model.ChangeTypeTest, Confidence: "high"}
	}
	if counts.docs == total {
		return model.ClassifyResult{Type: model.ChangeTypeDocs, Confidence: "high"}
	}
	if counts.deps == total {
		return model.ClassifyResult{Type: model.ChangeTypeDeps, Confidence: "high"}
	}
	if counts.config == total {
		return model.ClassifyResult{Type: model.ChangeTypeConfig, Confidence: "high"}
	}

	titleLower := strings.ToLower(meta.Title)
	branchLower := strings.ToLower(meta.HeadBranch)

	if containsAny(titleLower, "fix", "bug", "patch", "hotfix") || containsAny(branchLower, "fix", "bug", "hotfix") {
		return model.ClassifyResult{Type: model.ChangeTypeBugfix, Confidence: "high"}
	}
	if containsAny(titleLower, "refactor", "cleanup", "clean up") || containsAny(branchLower, "refactor") {
		return model.ClassifyResult{Type: model.ChangeTypeRefactor, Confidence: "medium"}
	}
	if containsAny(titleLower, "feat", "add", "implement", "introduce") || containsAny(branchLower, "feat", "feature") {
		return model.ClassifyResult{Type: model.ChangeTypeFeature, Confidence: "high"}
	}

	if counts.impl > 0 && counts.test > 0 {
		return model.ClassifyResult{Type: model.ChangeTypeFeature, Confidence: "medium"}
	}
	if counts.impl > 0 {
		return model.ClassifyResult{Type: model.ChangeTypeFeature, Confidence: "low"}
	}

	return model.ClassifyResult{Type: model.ChangeTypeMixed, Confidence: "low"}
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
