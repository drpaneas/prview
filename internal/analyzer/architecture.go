package analyzer

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/drpaneas/prview/internal/model"
)

func AnalyzeArchitecture(diffs []model.FileDiff, baseFiles, headFiles []model.FileContent, astResult model.ASTResult) model.ArchResult {
	result := model.ArchResult{}

	result.NewPackages = findNewPackages(diffs)
	result.NewDeps = findNewDeps(diffs)

	basePkgs := groupByPackage(baseFiles)
	headPkgs := groupByPackage(headFiles)

	var beforeChanges []model.ArchChange
	var afterChanges []model.ArchChange

	modifiedPkgs := map[string]bool{}
	for _, d := range diffs {
		pkg := filepath.Dir(d.Path)
		modifiedPkgs[pkg] = true
	}

	for pkg := range modifiedPkgs {
		baseFuncs := listExportedFuncs(basePkgs[pkg])
		headFuncs := listExportedFuncs(headPkgs[pkg])

		if len(baseFuncs) > 0 {
			beforeChanges = append(beforeChanges, model.ArchChange{
				Description: fmt.Sprintf("Package %s", pkg),
				Details:     baseFuncs,
			})
		}
		if len(headFuncs) > 0 {
			afterChanges = append(afterChanges, model.ArchChange{
				Description: fmt.Sprintf("Package %s", pkg),
				Details:     headFuncs,
			})
		}
	}

	result.Before = beforeChanges
	result.After = afterChanges

	result.DesignDecisions = inferDesignDecisions(diffs, astResult, result.NewDeps)

	return result
}

func findNewPackages(diffs []model.FileDiff) []string {
	pkgs := map[string]bool{}
	for _, d := range diffs {
		if d.Status == "added" {
			pkg := filepath.Dir(d.Path)
			pkgs[pkg] = true
		}
	}

	var result []string
	for pkg := range pkgs {
		allNew := true
		for _, d := range diffs {
			if filepath.Dir(d.Path) == pkg && d.Status != "added" {
				allNew = false
				break
			}
		}
		if allNew {
			result = append(result, pkg)
		}
	}
	return result
}

func findNewDeps(diffs []model.FileDiff) []string {
	var deps []string
	for _, d := range diffs {
		if filepath.Base(d.Path) != "go.mod" {
			continue
		}
		lines := strings.Split(d.Patch, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
				trimmed := strings.TrimSpace(line[1:])
				if strings.Contains(trimmed, "/") && !strings.HasPrefix(trimmed, "module") && !strings.HasPrefix(trimmed, "go ") {
					deps = append(deps, trimmed)
				}
			}
		}
	}
	return deps
}

func groupByPackage(files []model.FileContent) map[string][]model.FileContent {
	result := map[string][]model.FileContent{}
	for _, f := range files {
		pkg := filepath.Dir(f.Path)
		result[pkg] = append(result[pkg], f)
	}
	return result
}

func listExportedFuncs(files []model.FileContent) []string {
	var funcs []string
	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".go") || strings.HasSuffix(f.Path, "_test.go") {
			continue
		}
		for _, fn := range parseFuncs(f.Content) {
			if fn.Exported {
				funcs = append(funcs, fn.Signature)
			}
		}
	}
	return funcs
}

func inferDesignDecisions(diffs []model.FileDiff, astResult model.ASTResult, newDeps []string) []string {
	var decisions []string

	if len(newDeps) > 0 {
		decisions = append(decisions, fmt.Sprintf("Introduces %d new dependency(ies)", len(newDeps)))
	}

	newInterfaces := 0
	for _, exp := range astResult.NewExports {
		if strings.Contains(exp, "type") {
			newInterfaces++
		}
	}
	if newInterfaces > 0 {
		decisions = append(decisions, fmt.Sprintf("Adds %d new exported type(s) to the public API", newInterfaces))
	}

	addedFuncs := 0
	modifiedFuncs := 0
	for _, fn := range astResult.Functions {
		switch fn.ChangeKind {
		case "added":
			addedFuncs++
		case "modified":
			modifiedFuncs++
		}
	}
	if addedFuncs > 0 {
		decisions = append(decisions, fmt.Sprintf("Adds %d new function(s)", addedFuncs))
	}
	if modifiedFuncs > 0 {
		decisions = append(decisions, fmt.Sprintf("Modifies %d existing function signature(s) (potential breaking change)", modifiedFuncs))
	}

	return decisions
}
