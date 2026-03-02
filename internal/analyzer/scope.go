package analyzer

import (
	"path/filepath"
	"sort"

	"github.com/drpaneas/prview/internal/model"
)

func AnalyzeScope(diffs []model.FileDiff) model.ScopeResult {
	dirs := map[string]int{}
	totalAdded := 0
	totalDeleted := 0

	for _, d := range diffs {
		totalAdded += d.Additions
		totalDeleted += d.Deletions
		dir := filepath.Dir(d.Path)
		dirs[dir] += d.Additions
	}

	var breakdown []model.DirStat
	for dir, lines := range dirs {
		breakdown = append(breakdown, model.DirStat{Dir: dir, LinesAdded: lines})
	}

	if totalAdded > 0 {
		for i := range breakdown {
			breakdown[i].Percentage = float64(breakdown[i].LinesAdded) / float64(totalAdded) * 100
		}
	}

	sort.Slice(breakdown, func(i, j int) bool {
		return breakdown[i].LinesAdded > breakdown[j].LinesAdded
	})

	complexity := computeComplexity(len(diffs), len(dirs), totalAdded)

	return model.ScopeResult{
		FilesChanged:  len(diffs),
		TotalAdded:    totalAdded,
		TotalDeleted:  totalDeleted,
		PackagesCount: len(dirs),
		DirBreakdown:  breakdown,
		Complexity:    complexity,
	}
}

func computeComplexity(files, packages, linesAdded int) int {
	score := 0.0
	score += float64(files) * 0.5
	score += float64(packages) * 2
	if linesAdded > 500 {
		score += 4
	} else if linesAdded > 200 {
		score += 3
	} else if linesAdded > 50 {
		score += 2
	} else {
		score += 1
	}
	if score > 10 {
		score = 10
	}
	return int(score)
}
