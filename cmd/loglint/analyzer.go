package main

import (
	"go/ast"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// unparen returns e with any enclosing parentheses stripped.
func unparen(e ast.Expr) ast.Expr {
	for {
		p, ok := e.(*ast.ParenExpr)
		if !ok {
			return e
		}
		e = p.X
	}
}

const Doc = `check for github.com/gopherd/log unfinished chain calls.`

var analyzer = &analysis.Analyzer{
	Name:     "loglint",
	Doc:      Doc,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

// flags
var funcs, methods stringSetFlag

func init() {
	var sb strings.Builder

	for i, f := range allFuncs {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString("github.com/gopherd/log.")
		sb.WriteString(f)
	}
	funcs.Set(sb.String())

	sb.Reset()
	for i, m := range allMethods {
		if i > 0 {
			sb.WriteByte(',')
		}
		ptr := "*"
		typ := "Fields"
		if j := strings.Index(m, "."); j > 0 {
			typ = m[:j]
			m = m[j+1:]
			ptr = ""
		}
		if strings.HasPrefix(typ, "*") {
			typ = typ[1:]
			ptr = "*"
		}
		sb.WriteString("(" + ptr + "github.com/gopherd/log." + typ + ").")
		sb.WriteString(m)
	}
	methods.Set(sb.String())
}

func run(pass *analysis.Pass) (interface{}, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.ExprStmt)(nil),
	}
	inspect.Preorder(nodeFilter, func(n ast.Node) {
		call, ok := unparen(n.(*ast.ExprStmt).X).(*ast.CallExpr)
		if !ok {
			return // not a call statement
		}
		fun := unparen(call.Fun)

		if pass.TypesInfo.Types[fun].IsType() {
			return // a conversion, not a call
		}

		selector, ok := fun.(*ast.SelectorExpr)
		if !ok {
			return // neither a method call nor a qualified ident
		}

		sel, ok := pass.TypesInfo.Selections[selector]
		if ok && sel.Kind() == types.MethodVal {
			// method (e.g. foo.String())
			sig := sel.Type().(*types.Signature)
			if obj, ok := sel.Obj().(*types.Func); ok {
				qname := "(" + sig.Recv().Type().String() + ")." + obj.Name()
				if methods[qname] {
					pass.Reportf(call.Lparen, "result of %s call not used", qname)
				}
			}
		} else if !ok {
			// package-qualified function (e.g. fmt.Errorf)
			obj := pass.TypesInfo.Uses[selector.Sel]
			if obj, ok := obj.(*types.Func); ok {
				qname := obj.Pkg().Path() + "." + obj.Name()
				if funcs[qname] {
					pass.Reportf(call.Lparen, "result of %s call not used", qname)
				}
			}
		}
	})
	return nil, nil
}

type stringSetFlag map[string]bool

func (ss *stringSetFlag) String() string {
	var items []string
	for item := range *ss {
		items = append(items, item)
	}
	sort.Strings(items)
	return strings.Join(items, ",")
}

func (ss *stringSetFlag) Set(s string) error {
	m := make(map[string]bool) // clobber previous value
	if s != "" {
		for _, name := range strings.Split(s, ",") {
			if name == "" {
				continue // TODO: report error? proceed?
			}
			m[name] = true
		}
	}
	*ss = m
	return nil
}
