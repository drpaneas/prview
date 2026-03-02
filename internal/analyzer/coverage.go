package analyzer

import (
	"strings"

	"github.com/drpaneas/prview/internal/model"
)

func AnalyzeCoverage(diffs []model.FileDiff, headFiles []model.FileContent, astResult model.ASTResult) model.CoverageResult {
	testFiles := map[string]string{}
	for _, f := range headFiles {
		if strings.HasSuffix(f.Path, "_test.go") {
			testFiles[f.Path] = f.Content
		}
	}

	changedTestFiles := map[string]bool{}
	for _, d := range diffs {
		if strings.HasSuffix(d.Path, "_test.go") {
			changedTestFiles[d.Path] = true
		}
	}

	testLines := 0
	implLines := 0
	for _, d := range diffs {
		if strings.HasSuffix(d.Path, "_test.go") {
			testLines += d.Additions
		} else if strings.HasSuffix(d.Path, ".go") {
			implLines += d.Additions
		}
	}

	var funcCoverage []model.FuncCoverage
	for _, fn := range astResult.Functions {
		if fn.ChangeKind == "deleted" {
			continue
		}
		if !strings.HasSuffix(fn.File, ".go") {
			continue
		}

		testFilePath := correspondingTestFile(fn.File)
		testContent, hasTestFile := testFiles[testFilePath]

		if !hasTestFile {
			funcCoverage = append(funcCoverage, model.FuncCoverage{
				FuncName: fn.Name,
				File:     fn.File,
				Status:   model.CoverageNoTestFile,
			})
			continue
		}

		testFunc := findTestForFunc(fn.Name, testContent)
		if testFunc != "" {
			funcCoverage = append(funcCoverage, model.FuncCoverage{
				FuncName: fn.Name,
				File:     fn.File,
				TestFile: testFilePath,
				TestFunc: testFunc,
				Status:   model.CoverageCovered,
			})
		} else {
			funcCoverage = append(funcCoverage, model.FuncCoverage{
				FuncName: fn.Name,
				File:     fn.File,
				TestFile: testFilePath,
				Status:   model.CoverageNotTested,
			})
		}
	}

	ratio := 0.0
	if implLines > 0 {
		ratio = float64(testLines) / float64(implLines) * 100
	}

	return model.CoverageResult{
		TestRatio: ratio,
		TestLines: testLines,
		ImplLines: implLines,
		Functions: funcCoverage,
	}
}

func correspondingTestFile(path string) string {
	if strings.HasSuffix(path, "_test.go") {
		return path
	}
	return strings.TrimSuffix(path, ".go") + "_test.go"
}

func findTestForFunc(funcName string, testContent string) string {
	// Strip receiver prefix for matching: "Foo.Bar" -> "Bar"
	name := funcName
	if idx := strings.LastIndex(funcName, "."); idx >= 0 {
		name = funcName[idx+1:]
	}

	lines := strings.Split(testContent, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "func Test") && strings.Contains(trimmed, name) {
			// Extract test function name
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				testName := parts[1]
				if idx := strings.Index(testName, "("); idx > 0 {
					testName = testName[:idx]
				}
				return testName
			}
		}
	}
	return ""
}
