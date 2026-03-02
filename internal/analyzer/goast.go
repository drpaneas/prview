package analyzer

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
	"unicode"

	"github.com/drpaneas/prview/internal/model"
)

type funcInfo struct {
	Name      string
	Signature string
	Exported  bool
}

func AnalyzeAST(baseFiles, headFiles []model.FileContent) model.ASTResult {
	baseFuncs := extractAllFuncs(baseFiles)
	headFuncs := extractAllFuncs(headFiles)

	var changes []model.FuncChange
	var newExports []string

	// Functions in head but not base -> added
	for key, hf := range headFuncs {
		if _, exists := baseFuncs[key]; !exists {
			changes = append(changes, model.FuncChange{
				Name:       hf.Name,
				File:       fileFromKey(key),
				Exported:   hf.Exported,
				ChangeKind: "added",
				Signature:  hf.Signature,
			})
			if hf.Exported {
				newExports = append(newExports, fmt.Sprintf("func %s  %s", hf.Name, fileFromKey(key)))
			}
		}
	}

	// Functions in both but with different signatures -> modified
	for key, bf := range baseFuncs {
		if hf, exists := headFuncs[key]; exists {
			if bf.Signature != hf.Signature {
				changes = append(changes, model.FuncChange{
					Name:       hf.Name,
					File:       fileFromKey(key),
					Exported:   hf.Exported,
					ChangeKind: "modified",
					Signature:  hf.Signature,
				})
			}
		}
	}

	// Functions in base but not head -> deleted
	for key, bf := range baseFuncs {
		if _, exists := headFuncs[key]; !exists {
			changes = append(changes, model.FuncChange{
				Name:       bf.Name,
				File:       fileFromKey(key),
				Exported:   bf.Exported,
				ChangeKind: "deleted",
				Signature:  bf.Signature,
			})
		}
	}

	// Also detect new exported types
	baseTypes := extractAllTypes(baseFiles)
	headTypes := extractAllTypes(headFiles)
	for key := range headTypes {
		if _, exists := baseTypes[key]; !exists {
			parts := strings.SplitN(key, "::", 2)
			if len(parts) == 2 && isExported(parts[1]) {
				newExports = append(newExports, fmt.Sprintf("type %s  %s", parts[1], parts[0]))
			}
		}
	}

	return model.ASTResult{
		Functions:  changes,
		NewExports: newExports,
	}
}

// key format: "file::FuncName"
func extractAllFuncs(files []model.FileContent) map[string]funcInfo {
	result := map[string]funcInfo{}
	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".go") || strings.HasSuffix(f.Path, "_test.go") {
			continue
		}
		funcs := parseFuncs(f.Content)
		for _, fn := range funcs {
			key := f.Path + "::" + fn.Name
			result[key] = fn
		}
	}
	return result
}

func parseFuncs(src string) []funcInfo {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		return nil
	}

	var funcs []funcInfo
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		name := fn.Name.Name
		if fn.Recv != nil && len(fn.Recv.List) > 0 {
			name = receiverType(fn.Recv.List[0].Type) + "." + name
		}
		sig := formatSignature(fn)
		funcs = append(funcs, funcInfo{
			Name:      name,
			Signature: sig,
			Exported:  isExported(fn.Name.Name),
		})
	}
	return funcs
}

func extractAllTypes(files []model.FileContent) map[string]bool {
	result := map[string]bool{}
	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".go") || strings.HasSuffix(f.Path, "_test.go") {
			continue
		}
		types := parseTypes(f.Content)
		for _, t := range types {
			result[f.Path+"::"+t] = true
		}
	}
	return result
}

func parseTypes(src string) []string {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		return nil
	}

	var types []string
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if ok {
				types = append(types, ts.Name.Name)
			}
		}
	}
	return types
}

func receiverType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return receiverType(t.X)
	case *ast.Ident:
		return t.Name
	default:
		return "?"
	}
}

func formatSignature(fn *ast.FuncDecl) string {
	var params []string
	if fn.Type.Params != nil {
		for _, p := range fn.Type.Params.List {
			typeStr := exprToString(p.Type)
			if len(p.Names) == 0 {
				params = append(params, typeStr)
			} else {
				for _, n := range p.Names {
					params = append(params, n.Name+" "+typeStr)
				}
			}
		}
	}

	var returns string
	if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
		var retParts []string
		for _, r := range fn.Type.Results.List {
			retParts = append(retParts, exprToString(r.Type))
		}
		if len(retParts) == 1 {
			returns = " " + retParts[0]
		} else {
			returns = " (" + strings.Join(retParts, ", ") + ")"
		}
	}

	name := fn.Name.Name
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		name = receiverType(fn.Recv.List[0].Type) + "." + name
	}

	return fmt.Sprintf("func %s(%s)%s", name, strings.Join(params, ", "), returns)
}

func exprToString(expr ast.Expr) string {
	fset := token.NewFileSet()
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, expr); err != nil {
		return fmt.Sprintf("%v", expr)
	}
	return buf.String()
}

func isExported(name string) bool {
	if name == "" {
		return false
	}
	return unicode.IsUpper(rune(name[0]))
}

func fileFromKey(key string) string {
	parts := strings.SplitN(key, "::", 2)
	if len(parts) == 2 {
		return parts[0]
	}
	return key
}
