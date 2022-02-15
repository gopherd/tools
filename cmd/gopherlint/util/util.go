package util

import (
	"go/ast"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"
)

type StringSetFlag map[string]bool

func (ss *StringSetFlag) String() string {
	var items []string
	for item := range *ss {
		items = append(items, item)
	}
	sort.Strings(items)
	return strings.Join(items, ",")
}

func (ss *StringSetFlag) Set(s string) error {
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

// unparen returns e with any enclosing parentheses stripped.
func Unparen(e ast.Expr) ast.Expr {
	for {
		p, ok := e.(*ast.ParenExpr)
		if !ok {
			return e
		}
		e = p.X
	}
}

func IsPointer(typ types.Type) bool {
	if types.IsInterface(typ) {
		return true
	}
	switch typ.(type) {
	case *types.Pointer, *types.Chan, *types.Map, *types.Signature:
		return true
	}
	return false
}

func IsGenericPointer(typ types.Type) bool {
	return !IsError(typ) && IsPointer(typ)
}

func IsError(typ types.Type) bool {
	return typ.String() == "error"
}

func IsNil(pass *analysis.Pass, expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return false
	}
	obj := pass.TypesInfo.ObjectOf(ident)
	if obj == nil {
		return false
	}
	_, isNil := obj.(*types.Nil)
	return isNil
}

func GetFuncName(pass *analysis.Pass, fun ast.Expr) string {
	funcObj, funcSign, _ := GetFunc(pass, fun)
	return GetFuncNameBySign(funcObj, funcSign)
}

func GetFuncNameBySign(funcObj *types.Func, funcSign *types.Signature) string {
	if funcSign == nil {
		return ""
	}
	isMethod := funcSign.Recv() != nil
	if isMethod {
		return "(" + funcSign.Recv().Type().String() + ")." + funcObj.Name()
	}
	if funcObj == nil {
		return ""
	}
	return funcObj.Pkg().Path() + "." + funcObj.Name()
}

func GetFunc(pass *analysis.Pass, fun ast.Expr) (*types.Func, *types.Signature, bool) {
	selector, ok := fun.(*ast.SelectorExpr)
	if !ok {
		if ident, ok := fun.(*ast.Ident); ok {
			if sel, ok := pass.TypesInfo.Uses[ident]; ok {
				if obj, ok := sel.(*types.Func); ok {
					return obj, sel.Type().(*types.Signature), false
				}
			}
		}
		return nil, nil, false
	}

	sel, ok := pass.TypesInfo.Selections[selector]
	if ok && sel.Kind() == types.MethodVal {
		// method (e.g. foo.String())
		if obj, ok := sel.Obj().(*types.Func); ok {
			return obj, obj.Type().(*types.Signature), true
		}
	} else if !ok {
		// package-qualified function (e.g. fmt.Errorf)
		sel := pass.TypesInfo.Uses[selector.Sel]
		if obj, ok := sel.(*types.Func); ok {
			return obj, sel.Type().(*types.Signature), false
		}
	}
	return nil, nil, false
}

func IsInterruptedStmt(pass *analysis.Pass, stmt ast.Stmt) bool {
	switch stmt.(type) {
	case *ast.ReturnStmt, *ast.BranchStmt:
		// return, break, continue, or goto found
		return true
	case *ast.ExprStmt:
		call, ok := Unparen(stmt.(*ast.ExprStmt).X).(*ast.CallExpr)
		if !ok {
			// not a call statement
			return false
		}
		fun := Unparen(call.Fun)
		if pass.TypesInfo.Types[fun].IsType() {
			return false
		}
		funcObj, _, _ := GetFunc(pass, fun)
		if funcObj == nil {
			if ident, ok := fun.(*ast.Ident); ok && ident.Name == "panic" {
				return true
			}
			return false
		}
		funcName := strings.ToLower(funcObj.Name())
		switch funcName {
		case "panic", "panicf", "fatal", "fatalf", "exit":
			return true
		}
	}
	return false
}

func IsProtobufFile(filename string) bool {
	return strings.HasSuffix(filename, ".pb.go")
}
