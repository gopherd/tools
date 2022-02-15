package unusedresult

import (
	"go/ast"
	"go/types"
	"sort"
	"strings"

	"github.com/gopherd/tools/cmd/gopherlint/util"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const Doc = `check for unused results of calls to functions that returned result is one of types.

This analyzer reports calls to certain functions in which the result of the call is ignored.
The set of types may be controlled using flags -types.`

var checkTypes stringSetFlag

func init() {
	const pkg = "github.com/gopherd/log"
	checkTypes.Set("*" + pkg + ".Context")
	Analyzer.Flags.Var(&checkTypes, "types",
		"comma-separated list of types must be used when it is the unique result of some function")
}

var Analyzer = &analysis.Analyzer{
	Name:     "unusedresult",
	Doc:      Doc,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.ExprStmt)(nil),
	}
	inspect.Preorder(nodeFilter, func(n ast.Node) {
		call, ok := util.Unparen(n.(*ast.ExprStmt).X).(*ast.CallExpr)
		if !ok {
			return // not a call statement
		}
		fun := util.Unparen(call.Fun)

		if pass.TypesInfo.Types[fun].IsType() {
			return // a conversion, not a call
		}

		selector, ok := fun.(*ast.SelectorExpr)
		if !ok {
			if ident, ok := fun.(*ast.Ident); ok {
				if sel, ok := pass.TypesInfo.Uses[ident]; ok {
					if obj, ok := sel.(*types.Func); ok {
						sig := sel.Type().(*types.Signature)
						if isUnusedResult(sig) {
							qname := obj.Pkg().Path() + "." + obj.Name()
							pass.Reportf(call.Lparen, "result of %s call not used", qname)
						}
					}
				}
			}
			return
		}

		sel, ok := pass.TypesInfo.Selections[selector]
		if ok && sel.Kind() == types.MethodVal {
			// method (e.g. foo.String())
			sig := sel.Type().(*types.Signature)
			if obj, ok := sel.Obj().(*types.Func); ok && isUnusedResult(sig) {
				qname := "(" + sig.Recv().Type().String() + ")." + obj.Name()
				pass.Reportf(call.Lparen, "result of %s call not used", qname)
			}
		} else if !ok {
			// package-qualified function (e.g. fmt.Errorf)
			sel := pass.TypesInfo.Uses[selector.Sel]
			if obj, ok := sel.(*types.Func); ok {
				sig := sel.Type().(*types.Signature)
				if isUnusedResult(sig) {
					qname := obj.Pkg().Path() + "." + obj.Name()
					pass.Reportf(call.Lparen, "result of %s call not used", qname)
				}
			}
		}
	})
	return nil, nil
}

func isUnusedResult(sig *types.Signature) bool {
	tup := sig.Results()
	if tup.Len() != 1 {
		return false
	}
	return checkTypes[tup.At(0).Type().String()]
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
	m := make(map[string]bool)
	if s != "" {
		for _, name := range strings.Split(s, ",") {
			if name == "" {
				continue
			}
			m[name] = true
		}
	}
	*ss = m
	return nil
}
