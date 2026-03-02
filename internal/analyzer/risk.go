package analyzer

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"

	"github.com/drpaneas/prview/internal/model"
)

var (
	ignoredErrPattern   = regexp.MustCompile(`_\s*,\s*\w+\s*:?=\s*\w+[\.\w]*\(|_\s*=\s*\w+[\.\w]*\(`)
	deferClosePattern   = regexp.MustCompile(`defer\s+\w+\.Close\(\)`)
	contextTodoPattern  = regexp.MustCompile(`context\.TODO\(\)`)
	goFuncPattern       = regexp.MustCompile(`go\s+func\s*\(`)
	hardcodedIPPattern  = regexp.MustCompile(`"\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`)
	hardcodedURLPattern = regexp.MustCompile(`"https?://[^"]+`)
)

func DetectRisks(diffs []model.FileDiff, headFiles []model.FileContent) []model.RiskFlag {
	var flags []model.RiskFlag

	for _, d := range diffs {
		if d.Patch == "" || !strings.HasSuffix(d.Path, ".go") {
			continue
		}
		if strings.HasSuffix(d.Path, "_test.go") {
			continue
		}

		flags = append(flags, scanPatch(d)...)
	}

	for _, f := range headFiles {
		if !strings.HasSuffix(f.Path, ".go") || strings.HasSuffix(f.Path, "_test.go") {
			continue
		}
		flags = append(flags, scanLargeFunctions(f)...)
	}

	for _, d := range diffs {
		if d.Additions == 0 && d.Deletions == 0 && d.Patch == "" {
			flags = append(flags, model.RiskFlag{
				Severity:    model.SeverityMedium,
				File:        d.Path,
				Description: "file touched but has zero diff (empty commit artifact?)",
			})
		}
	}

	return flags
}

func scanPatch(d model.FileDiff) []model.RiskFlag {
	var flags []model.RiskFlag
	lines := strings.Split(d.Patch, "\n")
	lineNum := 0

	for _, line := range lines {
		if strings.HasPrefix(line, "@@") {
			lineNum = parseHunkStart(line)
			continue
		}
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			lineNum++
			added := line[1:]

			if ignoredErrPattern.MatchString(added) {
				flags = append(flags, model.RiskFlag{
					Severity:    model.SeverityHigh,
					File:        d.Path,
					Line:        lineNum,
					Description: "error return value ignored",
					Code:        strings.TrimSpace(added),
				})
			}

			if deferClosePattern.MatchString(added) {
				flags = append(flags, model.RiskFlag{
					Severity:    model.SeverityMedium,
					File:        d.Path,
					Line:        lineNum,
					Description: "error from Close() silently discarded in defer",
					Code:        strings.TrimSpace(added),
				})
			}

			if contextTodoPattern.MatchString(added) {
				flags = append(flags, model.RiskFlag{
					Severity:    model.SeverityMedium,
					File:        d.Path,
					Line:        lineNum,
					Description: "context.TODO() in production code - should accept context from caller",
					Code:        strings.TrimSpace(added),
				})
			}

			if goFuncPattern.MatchString(added) {
				flags = append(flags, model.RiskFlag{
					Severity:    model.SeverityMedium,
					File:        d.Path,
					Line:        lineNum,
					Description: "goroutine spawned - verify shutdown/cancellation mechanism",
					Code:        strings.TrimSpace(added),
				})
			}

			if hardcodedIPPattern.MatchString(added) || hardcodedURLPattern.MatchString(added) {
				flags = append(flags, model.RiskFlag{
					Severity:    model.SeverityMedium,
					File:        d.Path,
					Line:        lineNum,
					Description: "hardcoded URL or IP address",
					Code:        strings.TrimSpace(added),
				})
			}
		} else if !strings.HasPrefix(line, "-") {
			lineNum++
		}
	}

	return flags
}

func parseHunkStart(hunkHeader string) int {
	// @@ -a,b +c,d @@
	parts := strings.Split(hunkHeader, "+")
	if len(parts) < 2 {
		return 0
	}
	numStr := parts[1]
	if idx := strings.Index(numStr, ","); idx > 0 {
		numStr = numStr[:idx]
	}
	if idx := strings.Index(numStr, " "); idx > 0 {
		numStr = numStr[:idx]
	}
	n := 0
	fmt.Sscanf(numStr, "%d", &n)
	return n - 1 // will be incremented on next added line
}

func scanLargeFunctions(f model.FileContent) []model.RiskFlag {
	var flags []model.RiskFlag

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, f.Path, f.Content, 0)
	if err != nil {
		return nil
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		start := fset.Position(fn.Body.Lbrace)
		end := fset.Position(fn.Body.Rbrace)
		bodyLines := end.Line - start.Line

		if bodyLines > 50 {
			name := fn.Name.Name
			if fn.Recv != nil && len(fn.Recv.List) > 0 {
				name = receiverType(fn.Recv.List[0].Type) + "." + name
			}
			flags = append(flags, model.RiskFlag{
				Severity:    model.SeverityLow,
				File:        f.Path,
				Line:        start.Line,
				Description: fmt.Sprintf("large function %s (%d lines)", name, bodyLines),
			})
		}
	}

	return flags
}
