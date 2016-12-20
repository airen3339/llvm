// Package astx implements utility functions for generating abstract syntax
// trees of LLVM IR modules.
package astx

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/llir/llvm/asm/internal/ast"
	"github.com/llir/llvm/asm/internal/token"
	"github.com/llir/llvm/internal/enc"
	"github.com/pkg/errors"
)

// TODO: Remove debug output.

// dbg is a logger which prefixes debug messages with the file name and line
// number of callees.
var dbg = log.New(os.Stdout, "", log.Lshortfile)

// === [ Modules ] =============================================================

// NewModule returns a new module based on the given top-level declarations.
func NewModule(decls interface{}) (*ast.Module, error) {
	var ds []TopLevelDecl
	switch decls := decls.(type) {
	case []TopLevelDecl:
		ds = decls
	case nil:
		// no top-level declarations.
	default:
		return nil, errors.Errorf("invalid top-level declaration list type; expected []astx.TopLevelDecl, got %T", decls)
	}
	module := &ast.Module{}
	for _, d := range ds {
		switch d := d.(type) {
		case *ast.NamedType:
			module.Types = append(module.Types, d)
		case *ast.Global:
			module.Globals = append(module.Globals, d)
		case *ast.Function:
			module.Funcs = append(module.Funcs, d)
		default:
			dbg.Printf("support for %T not yet implemented", d)
		}
	}
	module = fixModule(module)

	// Translate the AST of the module to an equivalent LLVM IR module.
	//m, err := translate(module)
	//if err != nil {
	//	return nil, errors.WithStack(err)
	//}
	return module, nil
}

// TopLevelDecl represents a top-level declaration.
type TopLevelDecl interface{}

// NewTopLevelDeclList returns a new top-level declaration list based on the
// given top-level declaration.
func NewTopLevelDeclList(decl interface{}) ([]TopLevelDecl, error) {
	d, ok := decl.(TopLevelDecl)
	if !ok {
		return nil, errors.Errorf("invalid top-level declaration type; expected astx.TopLevelDecl, got %T", decl)
	}
	return []TopLevelDecl{d}, nil
}

// AppendTopLevelDecl appends the given top-level declaration to the top-level
// declaration list.
func AppendTopLevelDecl(decls, decl interface{}) ([]TopLevelDecl, error) {
	ds, ok := decls.([]TopLevelDecl)
	if !ok {
		return nil, errors.Errorf("invalid top-level declaration list type; expected []astx.TopLevelDecl, got %T", decls)
	}
	d, ok := decl.(TopLevelDecl)
	if !ok {
		return nil, errors.Errorf("invalid top-level declaration type; expected astx.TopLevelDecl, got %T", decl)
	}
	return append(ds, d), nil
}

// === [ Type definitions ] ====================================================

// NewTypeDef returns a new type definition based on the given type name and
// definition.
func NewTypeDef(name, typ interface{}) (*ast.NamedType, error) {
	n, ok := name.(*LocalIdent)
	if !ok {
		return nil, errors.Errorf("invalid type name type; expected *astx.LocalIdent, got %T", name)
	}
	var t ast.Type
	switch typ := typ.(type) {
	case ast.Type:
		t = typ
	case nil:
		// opaque identified struct type.
	default:
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", typ)
	}
	return &ast.NamedType{Name: n.name, Def: t}, nil
}

// === [ Global variables ] ====================================================

// NewGlobalDecl returns a new global variable declaration based on the given
// global variable name, immutability and type.
func NewGlobalDecl(name, immutable, typ interface{}) (*ast.Global, error) {
	n, ok := name.(*GlobalIdent)
	if !ok {
		return nil, errors.Errorf("invalid global name type; expected *astx.GlobalIdent, got %T", name)
	}
	imm, ok := immutable.(bool)
	if !ok {
		return nil, errors.Errorf("invalid immutability type; expected bool, got %T", immutable)
	}
	t, ok := typ.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid content type; expected ast.Type, got %T", typ)
	}
	global := &ast.Global{Name: n.name, Content: t}
	global.Immutable = imm
	return global, nil
}

// NewGlobalDef returns a new global variable definition based on the given
// global variable name, immutability, type and value.
func NewGlobalDef(name, immutable, typ, val interface{}) (*ast.Global, error) {
	n, ok := name.(*GlobalIdent)
	if !ok {
		return nil, errors.Errorf("invalid global name type; expected *astx.GlobalIdent, got %T", name)
	}
	imm, ok := immutable.(bool)
	if !ok {
		return nil, errors.Errorf("invalid immutability type; expected bool, got %T", immutable)
	}
	t, ok := typ.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", typ)
	}
	init, err := NewValue(t, val)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	i, ok := init.(ast.Constant)
	if !ok {
		return nil, errors.Errorf("invalid init type; expected ast.Constant, got %T", init)
	}
	global := &ast.Global{Name: n.name, Content: t, Init: i}
	global.Immutable = imm
	return global, nil
}

// === [ Functions ] ===========================================================

// NewFunctionDecl returns a new function declaration based on the given
// return type, function name and parameters.
func NewFunctionDecl(ret, name, params interface{}) (*ast.Function, error) {
	r, ok := ret.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid function return type; expected ast.Type, got %T", ret)
	}
	n, ok := name.(*GlobalIdent)
	if !ok {
		return nil, errors.Errorf("invalid function name type; expected *astx.GlobalIdent, got %T", name)
	}
	sig := &ast.FuncType{Ret: r}
	switch ps := params.(type) {
	case *Params:
		for _, param := range ps.params {
			sig.Params = append(sig.Params, param)
		}
		sig.Variadic = ps.variadic
	case nil:
		// no parameters.
	default:
		return nil, errors.Errorf("invalid function parameters type; expected *astx.Params or nil, got %T", params)
	}
	f := &ast.Function{
		Name: n.name,
		Sig:  sig,
	}
	return f, nil
}

// NewFunctionDef returns a new function definition based on the given function
// header and body.
func NewFunctionDef(header, body interface{}) (*ast.Function, error) {
	f, ok := header.(*ast.Function)
	if !ok {
		return nil, errors.Errorf("invalid function header type; expected *ast.Function, got %T", header)
	}
	blocks, ok := body.([]*ast.BasicBlock)
	if !ok {
		return nil, errors.Errorf("invalid function body type; expected []*ast.BasicBlock, got %T", body)
	}
	f.Blocks = blocks
	return f, nil
}

// Params represents a function parameters specifier.
type Params struct {
	// Function parameter types.
	params []*ast.Param
	// Variadicity of the function type.
	variadic bool
}

// NewParams returns a new function parameters specifier, based on the given
// function parameters and variadicity.
func NewParams(params interface{}, variadic bool) (*Params, error) {
	switch params := params.(type) {
	case []*ast.Param:
		return &Params{params: params, variadic: variadic}, nil
	case nil:
		return &Params{variadic: variadic}, nil
	default:
		return nil, errors.Errorf("invalid function parameter list; expected []*ast.Param or nil, got %T", params)
	}
}

// NewParamList returns a new function parameter list based on the given
// function parameter.
func NewParamList(param interface{}) ([]*ast.Param, error) {
	p, ok := param.(*ast.Param)
	if !ok {
		return nil, errors.Errorf("invalid function parameter type; expected *ast.Param, got %T", param)
	}
	return []*ast.Param{p}, nil
}

// AppendParam appends the given parameter to the function parameter list.
func AppendParam(params, param interface{}) ([]*ast.Param, error) {
	ps, ok := params.([]*ast.Param)
	if !ok {
		return nil, errors.Errorf("invalid function parameter list type; expected []*ast.Param, got %T", params)
	}
	p, ok := param.(*ast.Param)
	if !ok {
		return nil, errors.Errorf("invalid function parameter type; expected *ast.Param, got %T", param)
	}
	return append(ps, p), nil
}

// NewParam returns a new function parameter based on the given parameter type
// and name.
func NewParam(typ, name interface{}) (*ast.Param, error) {
	t, ok := typ.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", typ)
	}
	var n string
	switch name := name.(type) {
	case *LocalIdent:
		n = name.name
	case nil:
		// unnamed function parameter.
	default:
		return nil, errors.Errorf("invalid local name type; expected *astx.LocalIdent or nil, got %T", name)
	}
	return &ast.Param{Name: n, Type: t}, nil
}

// === [ Identifiers ] =========================================================

// GlobalIdent represents a global identifier.
type GlobalIdent struct {
	// Global identifier name the without "@" prefix.
	name string
}

// NewGlobalIdent returns a new global identifier based on the given global
// identifier token.
func NewGlobalIdent(ident interface{}) (*GlobalIdent, error) {
	s, err := getTokenString(ident)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if !strings.HasPrefix(s, "@") {
		return nil, errors.Errorf(`invalid global identifier %q; missing "@" prefix`, s)
	}
	s = s[1:]
	return &GlobalIdent{name: s}, nil
}

// LocalIdent represents a local identifier.
type LocalIdent struct {
	// Local identifier name the without "%" prefix.
	name string
}

// NewLocalIdent returns a new local identifier based on the given local
// identifier token.
func NewLocalIdent(ident interface{}) (*LocalIdent, error) {
	s, err := getTokenString(ident)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if !strings.HasPrefix(s, "%") {
		return nil, errors.Errorf(`invalid local identifier %q; missing "%%" prefix`, s)
	}
	s = s[1:]
	return &LocalIdent{name: s}, nil
}

// LabelIdent represents a label identifier.
type LabelIdent struct {
	// Label identifier name the without ":" suffix.
	name string
}

// NewLabelIdent returns a new label identifier based on the given label
// identifier token.
func NewLabelIdent(ident interface{}) (*LabelIdent, error) {
	s, err := getTokenString(ident)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if !strings.HasSuffix(s, ":") {
		return nil, errors.Errorf(`invalid label identifier %q; missing ":" suffix`, s)
	}
	s = s[:len(s)-1]
	return &LabelIdent{name: s}, nil
}

// === [ Types ] ===============================================================

// NewTypeList returns a new type list based on the given type.
func NewTypeList(typ interface{}) ([]ast.Type, error) {
	t, ok := typ.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", typ)
	}
	return []ast.Type{t}, nil
}

// AppendType appends the given type to the type list.
func AppendType(typs, typ interface{}) ([]ast.Type, error) {
	ts, ok := typs.([]ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type list type; expected []ast.Type, got %T", typs)
	}
	t, ok := typ.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", typ)
	}
	return append(ts, t), nil
}

// NewIntType returns a new integer type based on the given integer type token.
func NewIntType(typeTok interface{}) (*ast.IntType, error) {
	s, err := getTokenString(typeTok)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if !strings.HasPrefix(s, "i") {
		return nil, errors.Errorf(`invalid integer type %q; missing "i" prefix`, s)
	}
	s = s[1:]
	size, err := strconv.Atoi(s)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.IntType{Size: size}, nil
}

// NewFuncType returns a new function type based on the given return type and
// function parameters.
func NewFuncType(ret, params interface{}) (*ast.FuncType, error) {
	r, ok := ret.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid function return type; expected ast.Type, got %T", ret)
	}
	sig := &ast.FuncType{Ret: r}
	switch ps := params.(type) {
	case *Params:
		for _, param := range ps.params {
			sig.Params = append(sig.Params, param)
		}
		sig.Variadic = ps.variadic
	case nil:
		// no parameters.
	default:
		return nil, errors.Errorf("invalid function parameters type; expected *astx.Params or nil, got %T", params)
	}
	return sig, nil
}

// NewPointerType returns a new pointer type based on the given element type.
func NewPointerType(elem, space interface{}) (*ast.PointerType, error) {
	e, ok := elem.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid element type; expected ast.Type, got %T", elem)
	}
	t := &ast.PointerType{Elem: e}
	switch space := space.(type) {
	case *AddrSpace:
		t.Space = space.space
	case nil:
		// no address space.
	default:
		return nil, errors.Errorf("invalid address space type; expected *astx.AddrSpace or nil, got %T", space)
	}
	return t, nil
}

// AddrSpace represents the address space of a pointer type.
type AddrSpace struct {
	// Address space.
	space int64
}

// NewAddrSpace returns a new address space pointer based on the given space.
func NewAddrSpace(space interface{}) (*AddrSpace, error) {
	s, err := getInt64(space)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &AddrSpace{space: s}, nil
}

// NewVectorType returns a new vector type based on the given vector length and
// element type.
func NewVectorType(len, elem interface{}) (*ast.VectorType, error) {
	e, ok := elem.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid element type; expected ast.Type, got %T", elem)
	}
	l, err := getInt64(len)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.VectorType{Elem: e, Len: l}, nil
}

// NewArrayType returns a new array type based on the given array length and
// element type.
func NewArrayType(len, elem interface{}) (*ast.ArrayType, error) {
	e, ok := elem.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid element type; expected ast.Type, got %T", elem)
	}
	l, err := getInt64(len)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.ArrayType{Elem: e, Len: l}, nil
}

// NewStructType returns a new struct type based on the given struct fields.
func NewStructType(fields interface{}) (*ast.StructType, error) {
	var fs []ast.Type
	switch fields := fields.(type) {
	case []ast.Type:
		fs = fields
	case nil:
		// no struct fields.
	default:
		return nil, errors.Errorf("invalid struct fields type; expected []ast.Type, got %T", fields)
	}
	return &ast.StructType{Fields: fs}, nil
}

// NewTypeIdent returns a new type identifier based on the given local
// identifier.
func NewTypeIdent(name interface{}) (*ast.NamedTypeDummy, error) {
	n, ok := name.(*LocalIdent)
	if !ok {
		return nil, errors.Errorf("invalid type name type; expected *astx.LocalIdent, got %T", name)
	}
	return &ast.NamedTypeDummy{Name: n.name}, nil
}

// === [ Values ] ==============================================================

// NewValueList returns a new value list based on the given
// value.
func NewValueList(val interface{}) ([]ast.Value, error) {
	v, ok := val.(ast.Value)
	if !ok {
		return nil, errors.Errorf("invalid value type; expected ast.Value, got %T", val)
	}
	return []ast.Value{v}, nil
}

// AppendValue appends the given value to the value list.
func AppendValue(vals, val interface{}) ([]ast.Value, error) {
	vs, ok := vals.([]ast.Value)
	if !ok {
		return nil, errors.Errorf("invalid value list type; expected []ast.Value, got %T", vals)
	}
	v, ok := val.(ast.Value)
	if !ok {
		return nil, errors.Errorf("invalid value type; expected ast.Value, got %T", val)
	}
	return append(vs, v), nil
}

// NewValue returns a value based on the given type and value.
func NewValue(typ, val interface{}) (ast.Value, error) {
	t, ok := typ.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid value type; expected ast.Type, got %T", typ)
	}
	switch val := val.(type) {
	case *LocalIdent:
		return &ast.LocalDummy{Name: val.name, Type: t}, nil
	case *GlobalIdent:
		return &ast.GlobalDummy{Name: val.name, Type: t}, nil
	case *IntLit:
		return &ast.IntConst{Type: t, Lit: val.lit}, nil
	case *FloatLit:
		return &ast.FloatConst{Type: t, Lit: val.lit}, nil
	case *NullLit:
		return &ast.NullConst{Type: t}, nil
	case *ZeroInitializerLit:
		return &ast.ZeroInitializerConst{Type: t}, nil

	// Store type of vector, array and struct constants and constant expressions,
	// so that it may be evaluated after type resolution.
	case *ast.VectorConst:
		// Vector constant type should not be known at this stage of parsing, as
		// they've been constructed from VectorConst literals.
		if val.Type != nil {
			return nil, errors.Errorf("invalid vector constant type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ArrayConst:
		// Array constant type should not be known at this stage of parsing, as
		// they've been constructed from ArrayConst literals.
		if val.Type != nil {
			return nil, errors.Errorf("invalid array constant type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.StructConst:
		// Struct constant type should not be known at this stage of parsing, as
		// they've been constructed from StructConst literals.
		if val.Type != nil {
			return nil, errors.Errorf("invalid struct constant type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil

	// Binary instructions
	case *ast.ExprAdd:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprAdd production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid add constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprFAdd:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprFAdd production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid fadd constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprSub:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprSub production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid sub constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprFSub:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprFSub production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid fsub constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprMul:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprMul production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid mul constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprFMul:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprFMul production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid fmul constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprUDiv:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprUDiv production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid udiv constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprSDiv:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprSDiv production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid sdiv constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprFDiv:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprFDiv production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid fdiv constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprURem:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprURem production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid urem constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprSRem:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprSRem production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid srem constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprFRem:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprFRem production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid frem constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil

	// Bitwise instructions
	case *ast.ExprShl:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprShl production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid shl constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprLShr:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprLShr production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid lshr constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprAShr:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprAShr production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid ashr constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprAnd:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprAnd production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid and constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprOr:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprOr production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid or constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprXor:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprXor production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid xor constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil

	// Memory instructions
	case *ast.ExprGetElementPtr:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprGetElementPtr production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid getelementptr constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil

	// Conversion instructions
	case *ast.ExprTrunc:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprTrunc production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid trunc constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprZExt:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprZExt production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid zext constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprSExt:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprSExt production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid sext constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprFPTrunc:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprFPTrunc production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid fptrunc constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprFPExt:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprFPExt production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid fpext constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprFPToUI:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprFPToUI production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid fptoui constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprFPToSI:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprFPToSI production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid fptosi constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprUIToFP:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprUIToFP production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid uitofp constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprSIToFP:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprSIToFP production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid sitofp constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprPtrToInt:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprPtrToInt production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid ptrtoint constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprIntToPtr:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprIntToPtr production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid inttoptr constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprBitCast:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprBitCast production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid bitcast constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprAddrSpaceCast:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprAddrSpaceCast production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid addrspacecast constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil

	// Other instructions
	case *ast.ExprICmp:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprICmp production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid icmp constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprFCmp:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprFCmp production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid fcmp constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	case *ast.ExprSelect:
		// Constant expression type should not be known at this stage of parsing,
		// as they've been constructed from ExprSelect production rules.
		if val.Type != nil {
			return nil, errors.Errorf("invalid select constant expression type, expected nil, got %T", val.Type)
		}
		val.Type = t
		return val, nil
	default:
		panic(fmt.Sprintf("support for value type %T not yet implemented", val))
	}
}

// === [ Constants ] ===========================================================

// NewConstantList returns a new constant list based on the given constant.
func NewConstantList(x interface{}) ([]ast.Constant, error) {
	c, ok := x.(ast.Constant)
	if !ok {
		return nil, errors.Errorf("invalid constant type; expected ast.Constant, got %T", x)
	}
	return []ast.Constant{c}, nil
}

// AppendConstant appends the given constant to the constant list.
func AppendConstant(xs, x interface{}) ([]ast.Constant, error) {
	cs, ok := xs.([]ast.Constant)
	if !ok {
		return nil, errors.Errorf("invalid constant list type; expected []ast.Constant, got %T", xs)
	}
	c, ok := x.(ast.Constant)
	if !ok {
		return nil, errors.Errorf("invalid constant type; expected ast.Constant, got %T", x)
	}
	return append(cs, c), nil
}

// NewConstant returns a constant based on the given type and value.
func NewConstant(typ, val interface{}) (ast.Constant, error) {
	v, err := NewValue(typ, val)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	c, ok := v.(ast.Constant)
	if !ok {
		return nil, errors.Errorf("invalid constant type; expected ast.Constant, got %T", v)
	}
	return c, nil
}

// IntLit represents an integer literal.
type IntLit struct {
	// Integer literal.
	lit string
}

// NewIntLit returns a new integer literal based on the given integer token.
func NewIntLit(tok interface{}) (*IntLit, error) {
	s, err := getTokenString(tok)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &IntLit{lit: s}, nil
}

// FloatLit represents an floating-point literal.
type FloatLit struct {
	// Floating-point literal.
	lit string
}

// NewFloatLit returns a new floating-point literal based on the given floating-point  token.
func NewFloatLit(tok interface{}) (*FloatLit, error) {
	s, err := getTokenString(tok)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &FloatLit{lit: s}, nil
}

// NullLit represents a null literal.
type NullLit struct {
}

// NewVectorConst returns a new vector constant based on the given elements.
func NewVectorConst(elems interface{}) (*ast.VectorConst, error) {
	es, ok := elems.([]ast.Constant)
	if !ok {
		return nil, errors.Errorf("invalid vector elements type; expected []ast.Constant, got %T", elems)
	}
	return &ast.VectorConst{Elems: es}, nil
}

// NewArrayConst returns a new array constant based on the given elements.
func NewArrayConst(elems interface{}) (*ast.ArrayConst, error) {
	es, ok := elems.([]ast.Constant)
	if !ok {
		return nil, errors.Errorf("invalid array elements type; expected []ast.Constant, got %T", elems)
	}
	return &ast.ArrayConst{Elems: es}, nil
}

// NewCharArrayConst returns a new character array constant based on the given
// string.
func NewCharArrayConst(str interface{}) (*ast.ArrayConst, error) {
	s, err := getTokenString(str)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// Skip double-quotes.
	s = s[1 : len(s)-1]
	s = enc.Unescape(s)
	var elems []ast.Constant
	for i := 0; i < len(s); i++ {
		// TODO: Validate that string(s[i]) works for the entire byte range 0-255.
		// Otherwise, use *big.Int to implement integer constants in package ast.
		elem := &ast.IntConst{Type: &ast.IntType{Size: 8}, Lit: string(s[i])}
		elems = append(elems, elem)
	}
	c := &ast.ArrayConst{Elems: elems}
	c.CharArray = true
	return c, nil
}

// NewStructConst returns a new struct constant based on the given fields.
func NewStructConst(fields interface{}) (*ast.StructConst, error) {
	fs, ok := fields.([]ast.Constant)
	if !ok {
		return nil, errors.Errorf("invalid struct fields type; expected []ast.Constant, got %T", fields)
	}
	return &ast.StructConst{Fields: fs}, nil
}

// ZeroInitializerLit represents a zeroinitializer literal.
type ZeroInitializerLit struct {
}

// --- [ Binary expressions ] --------------------------------------------------

// NewAddExpr returns a new add expression based on the given type and operands.
func NewAddExpr(xTyp, xVal, yTyp, yVal interface{}) (*ast.ExprAdd, error) {
	x, err := NewConstant(xTyp, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewConstant(yTyp, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.ExprAdd{X: x, Y: y}, nil
}

// NewFAddExpr returns a new fadd expression based on the given type and
// operands.
func NewFAddExpr(xTyp, xVal, yTyp, yVal interface{}) (*ast.ExprFAdd, error) {
	x, err := NewConstant(xTyp, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewConstant(yTyp, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.ExprFAdd{X: x, Y: y}, nil
}

// NewSubExpr returns a new sub expression based on the given type and operands.
func NewSubExpr(xTyp, xVal, yTyp, yVal interface{}) (*ast.ExprSub, error) {
	x, err := NewConstant(xTyp, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewConstant(yTyp, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.ExprSub{X: x, Y: y}, nil
}

// NewFSubExpr returns a new fsub expression based on the given type and
// operands.
func NewFSubExpr(xTyp, xVal, yTyp, yVal interface{}) (*ast.ExprFSub, error) {
	x, err := NewConstant(xTyp, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewConstant(yTyp, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.ExprFSub{X: x, Y: y}, nil
}

// NewMulExpr returns a new mul expression based on the given type and operands.
func NewMulExpr(xTyp, xVal, yTyp, yVal interface{}) (*ast.ExprMul, error) {
	x, err := NewConstant(xTyp, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewConstant(yTyp, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.ExprMul{X: x, Y: y}, nil
}

// NewFMulExpr returns a new fmul expression based on the given type and
// operands.
func NewFMulExpr(xTyp, xVal, yTyp, yVal interface{}) (*ast.ExprFMul, error) {
	x, err := NewConstant(xTyp, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewConstant(yTyp, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.ExprFMul{X: x, Y: y}, nil
}

// NewUDivExpr returns a new udiv expression based on the given type and
// operands.
func NewUDivExpr(xTyp, xVal, yTyp, yVal interface{}) (*ast.ExprUDiv, error) {
	x, err := NewConstant(xTyp, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewConstant(yTyp, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.ExprUDiv{X: x, Y: y}, nil
}

// NewSDivExpr returns a new sdiv expression based on the given type and
// operands.
func NewSDivExpr(xTyp, xVal, yTyp, yVal interface{}) (*ast.ExprSDiv, error) {
	x, err := NewConstant(xTyp, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewConstant(yTyp, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.ExprSDiv{X: x, Y: y}, nil
}

// NewFDivExpr returns a new fdiv expression based on the given type and
// operands.
func NewFDivExpr(xTyp, xVal, yTyp, yVal interface{}) (*ast.ExprFDiv, error) {
	x, err := NewConstant(xTyp, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewConstant(yTyp, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.ExprFDiv{X: x, Y: y}, nil
}

// NewURemExpr returns a new urem expression based on the given type and
// operands.
func NewURemExpr(xTyp, xVal, yTyp, yVal interface{}) (*ast.ExprURem, error) {
	x, err := NewConstant(xTyp, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewConstant(yTyp, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.ExprURem{X: x, Y: y}, nil
}

// NewSRemExpr returns a new srem expression based on the given type and
// operands.
func NewSRemExpr(xTyp, xVal, yTyp, yVal interface{}) (*ast.ExprSRem, error) {
	x, err := NewConstant(xTyp, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewConstant(yTyp, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.ExprSRem{X: x, Y: y}, nil
}

// NewFRemExpr returns a new frem expression based on the given type and
// operands.
func NewFRemExpr(xTyp, xVal, yTyp, yVal interface{}) (*ast.ExprFRem, error) {
	x, err := NewConstant(xTyp, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewConstant(yTyp, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.ExprFRem{X: x, Y: y}, nil
}

// --- [ Bitwise expressions ] -------------------------------------------------

// NewShlExpr returns a new shl expression based on the given type and operands.
func NewShlExpr(xTyp, xVal, yTyp, yVal interface{}) (*ast.ExprShl, error) {
	x, err := NewConstant(xTyp, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewConstant(yTyp, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.ExprShl{X: x, Y: y}, nil
}

// NewLShrExpr returns a new lshr expression based on the given type and
// operands.
func NewLShrExpr(xTyp, xVal, yTyp, yVal interface{}) (*ast.ExprLShr, error) {
	x, err := NewConstant(xTyp, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewConstant(yTyp, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.ExprLShr{X: x, Y: y}, nil
}

// NewAShrExpr returns a new ashr expression based on the given type and
// operands.
func NewAShrExpr(xTyp, xVal, yTyp, yVal interface{}) (*ast.ExprAShr, error) {
	x, err := NewConstant(xTyp, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewConstant(yTyp, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.ExprAShr{X: x, Y: y}, nil
}

// NewAndExpr returns a new and expression based on the given type and operands.
func NewAndExpr(xTyp, xVal, yTyp, yVal interface{}) (*ast.ExprAnd, error) {
	x, err := NewConstant(xTyp, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewConstant(yTyp, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.ExprAnd{X: x, Y: y}, nil
}

// NewOrExpr returns a new or expression based on the given type and operands.
func NewOrExpr(xTyp, xVal, yTyp, yVal interface{}) (*ast.ExprOr, error) {
	x, err := NewConstant(xTyp, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewConstant(yTyp, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.ExprOr{X: x, Y: y}, nil
}

// NewXorExpr returns a new xor expression based on the given type and operands.
func NewXorExpr(xTyp, xVal, yTyp, yVal interface{}) (*ast.ExprXor, error) {
	x, err := NewConstant(xTyp, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewConstant(yTyp, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.ExprXor{X: x, Y: y}, nil
}

// --- [ Memory expressions ] --------------------------------------------------

// NewGetElementPtrExpr returns a new getelementptr expression based on the
// given element type, source address type and value, and element indices.
func NewGetElementPtrExpr(elem, srcTyp, srcVal, indices interface{}) (*ast.ExprGetElementPtr, error) {
	e, ok := elem.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid element type; expected ast.Type, got %T", elem)
	}
	src, err := NewConstant(srcTyp, srcVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	var is []ast.Constant
	switch indices := indices.(type) {
	case []ast.Constant:
		is = indices
	case nil:
		// no indices.
	default:
		return nil, errors.Errorf("invalid indices type; expected []ast.Constant or nil, got %T", indices)
	}
	return &ast.ExprGetElementPtr{Elem: e, Src: src, Indices: is}, nil
}

// --- [ Conversion expressions ] ----------------------------------------------

// NewTruncExpr returns a new trunc expression based on the given source value
// and target type.
func NewTruncExpr(fromTyp, fromVal, to interface{}) (*ast.ExprTrunc, error) {
	from, err := NewConstant(fromTyp, fromVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	t, ok := to.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", to)
	}
	return &ast.ExprTrunc{From: from, To: t}, nil
}

// NewZExtExpr returns a new zext expression based on the given source value and
// target type.
func NewZExtExpr(fromTyp, fromVal, to interface{}) (*ast.ExprZExt, error) {
	from, err := NewConstant(fromTyp, fromVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	t, ok := to.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", to)
	}
	return &ast.ExprZExt{From: from, To: t}, nil
}

// NewSExtExpr returns a new sext expression based on the given source value and
// target type.
func NewSExtExpr(fromTyp, fromVal, to interface{}) (*ast.ExprSExt, error) {
	from, err := NewConstant(fromTyp, fromVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	t, ok := to.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", to)
	}
	return &ast.ExprSExt{From: from, To: t}, nil
}

// NewFPTruncExpr returns a new fptrunc expression based on the given source
// value and target type.
func NewFPTruncExpr(fromTyp, fromVal, to interface{}) (*ast.ExprFPTrunc, error) {
	from, err := NewConstant(fromTyp, fromVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	t, ok := to.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", to)
	}
	return &ast.ExprFPTrunc{From: from, To: t}, nil
}

// NewFPExtExpr returns a new fpext expression based on the given source value
// and target type.
func NewFPExtExpr(fromTyp, fromVal, to interface{}) (*ast.ExprFPExt, error) {
	from, err := NewConstant(fromTyp, fromVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	t, ok := to.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", to)
	}
	return &ast.ExprFPExt{From: from, To: t}, nil
}

// NewFPToUIExpr returns a new fptoui expression based on the given source value
// and target type.
func NewFPToUIExpr(fromTyp, fromVal, to interface{}) (*ast.ExprFPToUI, error) {
	from, err := NewConstant(fromTyp, fromVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	t, ok := to.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", to)
	}
	return &ast.ExprFPToUI{From: from, To: t}, nil
}

// NewFPToSIExpr returns a new fptosi expression based on the given source value
// and target type.
func NewFPToSIExpr(fromTyp, fromVal, to interface{}) (*ast.ExprFPToSI, error) {
	from, err := NewConstant(fromTyp, fromVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	t, ok := to.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", to)
	}
	return &ast.ExprFPToSI{From: from, To: t}, nil
}

// NewUIToFPExpr returns a new uitofp expression based on the given source value
// and target type.
func NewUIToFPExpr(fromTyp, fromVal, to interface{}) (*ast.ExprUIToFP, error) {
	from, err := NewConstant(fromTyp, fromVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	t, ok := to.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", to)
	}
	return &ast.ExprUIToFP{From: from, To: t}, nil
}

// NewSIToFPExpr returns a new sitofp expression based on the given source value
// and target type.
func NewSIToFPExpr(fromTyp, fromVal, to interface{}) (*ast.ExprSIToFP, error) {
	from, err := NewConstant(fromTyp, fromVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	t, ok := to.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", to)
	}
	return &ast.ExprSIToFP{From: from, To: t}, nil
}

// NewPtrToIntExpr returns a new ptrtoint expression based on the given source
// value and target type.
func NewPtrToIntExpr(fromTyp, fromVal, to interface{}) (*ast.ExprPtrToInt, error) {
	from, err := NewConstant(fromTyp, fromVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	t, ok := to.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", to)
	}
	return &ast.ExprPtrToInt{From: from, To: t}, nil
}

// NewIntToPtrExpr returns a new inttoptr expression based on the given source
// value and target type.
func NewIntToPtrExpr(fromTyp, fromVal, to interface{}) (*ast.ExprIntToPtr, error) {
	from, err := NewConstant(fromTyp, fromVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	t, ok := to.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", to)
	}
	return &ast.ExprIntToPtr{From: from, To: t}, nil
}

// NewBitCastExpr returns a new bitcast expression based on the given source
// value and target type.
func NewBitCastExpr(fromTyp, fromVal, to interface{}) (*ast.ExprBitCast, error) {
	from, err := NewConstant(fromTyp, fromVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	t, ok := to.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", to)
	}
	return &ast.ExprBitCast{From: from, To: t}, nil
}

// NewAddrSpaceCastExpr returns a new addrspacecast expression based on the
// given source value and target type.
func NewAddrSpaceCastExpr(fromTyp, fromVal, to interface{}) (*ast.ExprAddrSpaceCast, error) {
	from, err := NewConstant(fromTyp, fromVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	t, ok := to.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", to)
	}
	return &ast.ExprAddrSpaceCast{From: from, To: t}, nil
}

// --- [ Other expressions ] ---------------------------------------------------

// NewICmpExpr returns a new icmp expression based on the given integer
// condition code, type and operands.
func NewICmpExpr(cond, xTyp, xVal, yTyp, yVal interface{}) (*ast.ExprICmp, error) {
	c, ok := cond.(ast.IntPred)
	if !ok {
		return nil, errors.Errorf("invalid integer predicate type; expected ast.IntPred, got %T", cond)
	}
	x, err := NewConstant(xTyp, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewConstant(yTyp, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.ExprICmp{Cond: c, X: x, Y: y}, nil
}

// NewFCmpExpr returns a new fcmp expression based on the given floating-point
// condition code, type and operands.
func NewFCmpExpr(cond, xTyp, xVal, yTyp, yVal interface{}) (*ast.ExprFCmp, error) {
	c, ok := cond.(ast.FloatPred)
	if !ok {
		return nil, errors.Errorf("invalid floating-point predicate type; expected ast.FloatPred, got %T", cond)
	}
	x, err := NewConstant(xTyp, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewConstant(yTyp, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.ExprFCmp{Cond: c, X: x, Y: y}, nil
}

// NewSeExpr returns a new select expression based on the given selection
// condition type and value, and operands.
func NewSelectExpr(condTyp, condVal, xTyp, xVal, yTyp, yVal interface{}) (*ast.ExprSelect, error) {
	cond, err := NewConstant(condTyp, condVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	x, err := NewConstant(xTyp, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewConstant(yTyp, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.ExprSelect{Cond: cond, X: x, Y: y}, nil
}

// === [ Basic blocks ] ========================================================

// NewBasicBlockList returns a new basic block list based on the given basic
// block.
func NewBasicBlockList(block interface{}) ([]*ast.BasicBlock, error) {
	b, ok := block.(*ast.BasicBlock)
	if !ok {
		return nil, errors.Errorf("invalid basic block type; expected *ast.BasicBlock, got %T", block)
	}
	return []*ast.BasicBlock{b}, nil
}

// AppendBasicBlock appends the given basic block to the basic block list.
func AppendBasicBlock(blocks, block interface{}) ([]*ast.BasicBlock, error) {
	bs, ok := blocks.([]*ast.BasicBlock)
	if !ok {
		return nil, errors.Errorf("invalid basic block list type; expected []*ast.BasicBlock, got %T", blocks)
	}
	b, ok := block.(*ast.BasicBlock)
	if !ok {
		return nil, errors.Errorf("invalid basic block type; expected *ast.BasicBlock, got %T", block)
	}
	return append(bs, b), nil
}

// NewBasicBlock returns a new basic block based on the given label name, non-
// branching instructions and terminator.
func NewBasicBlock(name, insts, term interface{}) (*ast.BasicBlock, error) {
	block := &ast.BasicBlock{}
	switch name := name.(type) {
	case *LabelIdent:
		block.Name = name.name
	case nil:
		// unnamed basic block.
	default:
		return nil, errors.Errorf("invalid label name type; expected *astx.LabelIdent or nil, got %T", name)
	}
	var is []ast.Instruction
	switch insts := insts.(type) {
	case []ast.Instruction:
		is = insts
	case nil:
		// no instructions.
	default:
		return nil, errors.Errorf("invalid instruction list type; expected []ast.Instruction, got %T", insts)
	}
	t, ok := term.(ast.Terminator)
	if !ok {
		return nil, errors.Errorf("invalid terminator type; expected ast.Terminator, got %T", term)
	}
	block.Insts = is
	block.Term = t
	return block, nil
}

// === [ Instructions ] ========================================================

// NewInstructionList returns a new instruction list based on the given
// instruction.
func NewInstructionList(inst interface{}) ([]ast.Instruction, error) {
	i, ok := inst.(ast.Instruction)
	if !ok {
		return nil, errors.Errorf("invalid instruction type; expected ast.Instruction, got %T", inst)
	}
	return []ast.Instruction{i}, nil
}

// AppendInstruction appends the given instruction to the instruction list.
func AppendInstruction(insts, inst interface{}) ([]ast.Instruction, error) {
	is, ok := insts.([]ast.Instruction)
	if !ok {
		return nil, errors.Errorf("invalid instruction list type; expected []ast.Instruction, got %T", insts)
	}
	i, ok := inst.(ast.Instruction)
	if !ok {
		return nil, errors.Errorf("invalid instruction type; expected ast.Instruction, got %T", inst)
	}
	return append(is, i), nil
}

// NewNamedInstruction returns a named instruction based on the given local
// variable name and instruction.
func NewNamedInstruction(name, inst interface{}) (ast.Instruction, error) {
	// namedInstruction represents a namedInstruction instruction.
	type namedInstruction interface {
		ast.Instruction
		ast.NamedValue
	}
	n, ok := name.(*LocalIdent)
	if !ok {
		return nil, errors.Errorf("invalid local variable name type; expected *astx.LocalIdent, got %T", name)
	}
	i, ok := inst.(namedInstruction)
	if !ok {
		return nil, errors.Errorf("invalid instruction type; expected namedInstruction, got %T", inst)
	}
	i.SetName(n.name)
	return i, nil
}

// --- [ Binary instructions ] -------------------------------------------------

// NewAddInst returns a new add instruction based on the given type and
// operands.
func NewAddInst(typ, xVal, yVal interface{}) (*ast.InstAdd, error) {
	x, err := NewValue(typ, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewValue(typ, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.InstAdd{X: x, Y: y}, nil
}

// NewFAddInst returns a new fadd instruction based on the given type and
// operands.
func NewFAddInst(typ, xVal, yVal interface{}) (*ast.InstFAdd, error) {
	x, err := NewValue(typ, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewValue(typ, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.InstFAdd{X: x, Y: y}, nil
}

// NewSubInst returns a new sub instruction based on the given type and
// operands.
func NewSubInst(typ, xVal, yVal interface{}) (*ast.InstSub, error) {
	x, err := NewValue(typ, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewValue(typ, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.InstSub{X: x, Y: y}, nil
}

// NewFSubInst returns a new fsub instruction based on the given type and
// operands.
func NewFSubInst(typ, xVal, yVal interface{}) (*ast.InstFSub, error) {
	x, err := NewValue(typ, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewValue(typ, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.InstFSub{X: x, Y: y}, nil
}

// NewMulInst returns a new mul instruction based on the given type and
// operands.
func NewMulInst(typ, xVal, yVal interface{}) (*ast.InstMul, error) {
	x, err := NewValue(typ, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewValue(typ, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.InstMul{X: x, Y: y}, nil
}

// NewFMulInst returns a new fmul instruction based on the given type and
// operands.
func NewFMulInst(typ, xVal, yVal interface{}) (*ast.InstFMul, error) {
	x, err := NewValue(typ, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewValue(typ, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.InstFMul{X: x, Y: y}, nil
}

// NewUDivInst returns a new udiv instruction based on the given type and
// operands.
func NewUDivInst(typ, xVal, yVal interface{}) (*ast.InstUDiv, error) {
	x, err := NewValue(typ, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewValue(typ, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.InstUDiv{X: x, Y: y}, nil
}

// NewSDivInst returns a new sdiv instruction based on the given type and
// operands.
func NewSDivInst(typ, xVal, yVal interface{}) (*ast.InstSDiv, error) {
	x, err := NewValue(typ, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewValue(typ, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.InstSDiv{X: x, Y: y}, nil
}

// NewFDivInst returns a new fdiv instruction based on the given type and
// operands.
func NewFDivInst(typ, xVal, yVal interface{}) (*ast.InstFDiv, error) {
	x, err := NewValue(typ, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewValue(typ, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.InstFDiv{X: x, Y: y}, nil
}

// NewURemInst returns a new urem instruction based on the given type and
// operands.
func NewURemInst(typ, xVal, yVal interface{}) (*ast.InstURem, error) {
	x, err := NewValue(typ, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewValue(typ, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.InstURem{X: x, Y: y}, nil
}

// NewSRemInst returns a new srem instruction based on the given type and
// operands.
func NewSRemInst(typ, xVal, yVal interface{}) (*ast.InstSRem, error) {
	x, err := NewValue(typ, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewValue(typ, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.InstSRem{X: x, Y: y}, nil
}

// NewFRemInst returns a new frem instruction based on the given type and
// operands.
func NewFRemInst(typ, xVal, yVal interface{}) (*ast.InstFRem, error) {
	x, err := NewValue(typ, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewValue(typ, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.InstFRem{X: x, Y: y}, nil
}

// --- [ Bitwise instructions ] ------------------------------------------------

// NewShlInst returns a new shl instruction based on the given type and
// operands.
func NewShlInst(typ, xVal, yVal interface{}) (*ast.InstShl, error) {
	x, err := NewValue(typ, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewValue(typ, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.InstShl{X: x, Y: y}, nil
}

// NewLShrInst returns a new lshr instruction based on the given type and
// operands.
func NewLShrInst(typ, xVal, yVal interface{}) (*ast.InstLShr, error) {
	x, err := NewValue(typ, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewValue(typ, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.InstLShr{X: x, Y: y}, nil
}

// NewAShrInst returns a new ashr instruction based on the given type and
// operands.
func NewAShrInst(typ, xVal, yVal interface{}) (*ast.InstAShr, error) {
	x, err := NewValue(typ, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewValue(typ, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.InstAShr{X: x, Y: y}, nil
}

// NewAndInst returns a new and instruction based on the given type and
// operands.
func NewAndInst(typ, xVal, yVal interface{}) (*ast.InstAnd, error) {
	x, err := NewValue(typ, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewValue(typ, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.InstAnd{X: x, Y: y}, nil
}

// NewOrInst returns a new or instruction based on the given type and
// operands.
func NewOrInst(typ, xVal, yVal interface{}) (*ast.InstOr, error) {
	x, err := NewValue(typ, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewValue(typ, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.InstOr{X: x, Y: y}, nil
}

// NewXorInst returns a new xor instruction based on the given type and
// operands.
func NewXorInst(typ, xVal, yVal interface{}) (*ast.InstXor, error) {
	x, err := NewValue(typ, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewValue(typ, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.InstXor{X: x, Y: y}, nil
}

// --- [ Memory instructions ] -------------------------------------------------

// NewAllocaInst returns a new alloca instruction based on the given element
// type and number of elements.
func NewAllocaInst(elem, nelems interface{}) (*ast.InstAlloca, error) {
	e, ok := elem.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid element type; expected ast.Type, got %T", elem)
	}
	inst := &ast.InstAlloca{Elem: e}
	switch nelems := nelems.(type) {
	case ast.Value:
		inst.NElems = nelems
	case nil:
		// no nelems.
	default:
		return nil, errors.Errorf("invalid number of elements type; expected ast.Value or nil, got %T", nelems)
	}
	return inst, nil
}

// NewLoadInst returns a new load instruction based on the given element type,
// source address type and value.
func NewLoadInst(elem, srcTyp, srcVal interface{}) (*ast.InstLoad, error) {
	e, ok := elem.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid element type; expected ast.Type, got %T", elem)
	}
	src, err := NewValue(srcTyp, srcVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// Store e in InstLoad to evaluate against src.Type().Elem() after type
	// resolution.
	return &ast.InstLoad{Elem: e, Src: src}, nil
}

// NewStoreInst returns a new store instruction based on the given element type,
// source address type and value.
func NewStoreInst(srcTyp, srcVal, dstTyp, dstVal interface{}) (*ast.InstStore, error) {
	src, err := NewValue(srcTyp, srcVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	dst, err := NewValue(dstTyp, dstVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.InstStore{Src: src, Dst: dst}, nil
}

// NewGetElementPtrInst returns a new getelementptr instruction based on the
// given element type, source address type and value, and element indices.
func NewGetElementPtrInst(elem, srcTyp, srcVal, indices interface{}) (*ast.InstGetElementPtr, error) {
	e, ok := elem.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid element type; expected ast.Type, got %T", elem)
	}
	src, err := NewValue(srcTyp, srcVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	var is []ast.Value
	switch indices := indices.(type) {
	case []ast.Value:
		is = indices
	case nil:
		// no indices.
	default:
		return nil, errors.Errorf("invalid indices type; expected []ast.Value or nil, got %T", indices)
	}
	// Store e in InstGetElementPtr to evaluate against src.Type().Elem() after
	// type resolution.
	return &ast.InstGetElementPtr{Elem: e, Src: src, Indices: is}, nil
}

// --- [ Conversion instructions ] ---------------------------------------------

// NewTruncInst returns a new trunc instruction based on the given source value
// and target type.
func NewTruncInst(fromTyp, fromVal, to interface{}) (*ast.InstTrunc, error) {
	from, err := NewValue(fromTyp, fromVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	t, ok := to.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", to)
	}
	return &ast.InstTrunc{From: from, To: t}, nil
}

// NewZExtInst returns a new zext instruction based on the given source value
// and target type.
func NewZExtInst(fromTyp, fromVal, to interface{}) (*ast.InstZExt, error) {
	from, err := NewValue(fromTyp, fromVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	t, ok := to.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", to)
	}
	return &ast.InstZExt{From: from, To: t}, nil
}

// NewSExtInst returns a new sext instruction based on the given source value
// and target type.
func NewSExtInst(fromTyp, fromVal, to interface{}) (*ast.InstSExt, error) {
	from, err := NewValue(fromTyp, fromVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	t, ok := to.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", to)
	}
	return &ast.InstSExt{From: from, To: t}, nil
}

// NewFPTruncInst returns a new fptrunc instruction based on the given source value
// and target type.
func NewFPTruncInst(fromTyp, fromVal, to interface{}) (*ast.InstFPTrunc, error) {
	from, err := NewValue(fromTyp, fromVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	t, ok := to.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", to)
	}
	return &ast.InstFPTrunc{From: from, To: t}, nil
}

// NewFPExtInst returns a new fpext instruction based on the given source value
// and target type.
func NewFPExtInst(fromTyp, fromVal, to interface{}) (*ast.InstFPExt, error) {
	from, err := NewValue(fromTyp, fromVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	t, ok := to.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", to)
	}
	return &ast.InstFPExt{From: from, To: t}, nil
}

// NewFPToUIInst returns a new fptoui instruction based on the given source value
// and target type.
func NewFPToUIInst(fromTyp, fromVal, to interface{}) (*ast.InstFPToUI, error) {
	from, err := NewValue(fromTyp, fromVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	t, ok := to.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", to)
	}
	return &ast.InstFPToUI{From: from, To: t}, nil
}

// NewFPToSIInst returns a new fptosi instruction based on the given source value
// and target type.
func NewFPToSIInst(fromTyp, fromVal, to interface{}) (*ast.InstFPToSI, error) {
	from, err := NewValue(fromTyp, fromVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	t, ok := to.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", to)
	}
	return &ast.InstFPToSI{From: from, To: t}, nil
}

// NewUIToFPInst returns a new uitofp instruction based on the given source value
// and target type.
func NewUIToFPInst(fromTyp, fromVal, to interface{}) (*ast.InstUIToFP, error) {
	from, err := NewValue(fromTyp, fromVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	t, ok := to.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", to)
	}
	return &ast.InstUIToFP{From: from, To: t}, nil
}

// NewSIToFPInst returns a new sitofp instruction based on the given source value
// and target type.
func NewSIToFPInst(fromTyp, fromVal, to interface{}) (*ast.InstSIToFP, error) {
	from, err := NewValue(fromTyp, fromVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	t, ok := to.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", to)
	}
	return &ast.InstSIToFP{From: from, To: t}, nil
}

// NewPtrToIntInst returns a new ptrtoint instruction based on the given source value
// and target type.
func NewPtrToIntInst(fromTyp, fromVal, to interface{}) (*ast.InstPtrToInt, error) {
	from, err := NewValue(fromTyp, fromVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	t, ok := to.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", to)
	}
	return &ast.InstPtrToInt{From: from, To: t}, nil
}

// NewIntToPtrInst returns a new inttoptr instruction based on the given source value
// and target type.
func NewIntToPtrInst(fromTyp, fromVal, to interface{}) (*ast.InstIntToPtr, error) {
	from, err := NewValue(fromTyp, fromVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	t, ok := to.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", to)
	}
	return &ast.InstIntToPtr{From: from, To: t}, nil
}

// NewBitCastInst returns a new bitcast instruction based on the given source value
// and target type.
func NewBitCastInst(fromTyp, fromVal, to interface{}) (*ast.InstBitCast, error) {
	from, err := NewValue(fromTyp, fromVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	t, ok := to.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", to)
	}
	return &ast.InstBitCast{From: from, To: t}, nil
}

// NewAddrSpaceCastInst returns a new addrspacecast instruction based on the given source value
// and target type.
func NewAddrSpaceCastInst(fromTyp, fromVal, to interface{}) (*ast.InstAddrSpaceCast, error) {
	from, err := NewValue(fromTyp, fromVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	t, ok := to.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", to)
	}
	return &ast.InstAddrSpaceCast{From: from, To: t}, nil
}

// --- [ Other instructions ] --------------------------------------------------

// NewICmpInst returns a new icmp instruction based on the given integer
// condition code, type and operands.
func NewICmpInst(cond, typ, xVal, yVal interface{}) (*ast.InstICmp, error) {
	c, ok := cond.(ast.IntPred)
	if !ok {
		return nil, errors.Errorf("invalid integer predicate type; expected ast.IntPred, got %T", cond)
	}
	x, err := NewValue(typ, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewValue(typ, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.InstICmp{Cond: c, X: x, Y: y}, nil
}

// NewFCmpInst returns a new fcmp instruction based on the given floating-point
// condition code, type and operands.
func NewFCmpInst(cond, typ, xVal, yVal interface{}) (*ast.InstFCmp, error) {
	c, ok := cond.(ast.FloatPred)
	if !ok {
		return nil, errors.Errorf("invalid floating-point predicate type; expected ast.FloatPred, got %T", cond)
	}
	x, err := NewValue(typ, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewValue(typ, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.InstFCmp{Cond: c, X: x, Y: y}, nil
}

// NewPhiInst returns a new phi instruction based on the given incoming values.
func NewPhiInst(typ, incs interface{}) (*ast.InstPhi, error) {
	t, ok := typ.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid type; expected ast.Type, got %T", typ)
	}
	is, ok := incs.([]*ast.Incoming)
	if !ok {
		return nil, errors.Errorf("invalid incoming value list type; expected []*ast.Incoming, got %T", incs)
	}
	for _, inc := range is {
		x, err := NewValue(typ, inc.X)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		inc.X = x
	}
	return &ast.InstPhi{Type: t, Incs: is}, nil
}

// NewIncomingList returns a new incoming value list based on the given incoming
// value.
func NewIncomingList(inc interface{}) ([]*ast.Incoming, error) {
	i, ok := inc.(*ast.Incoming)
	if !ok {
		return nil, errors.Errorf("invalid incoming value type; expected *ast.Incoming, got %T", inc)
	}
	return []*ast.Incoming{i}, nil
}

// AppendIncoming appends the given incoming value to the incoming value list.
func AppendIncoming(incs, inc interface{}) ([]*ast.Incoming, error) {
	is, ok := incs.([]*ast.Incoming)
	if !ok {
		return nil, errors.Errorf("invalid incoming value list type; expected []*ast.Incoming, got %T", incs)
	}
	i, ok := inc.(*ast.Incoming)
	if !ok {
		return nil, errors.Errorf("invalid incoming value type; expected *ast.Incoming, got %T", inc)
	}
	return append(is, i), nil
}

// NewIncoming returns a new incoming value based on the given value and
// predecessor basic block.
func NewIncoming(x, pred interface{}) (*ast.Incoming, error) {
	pp, err := NewValue(&ast.TypeDummy{}, pred)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	p, ok := pp.(ast.NamedValue)
	if !ok {
		return nil, errors.Errorf("invalid predecessor type; expected ast.NamedValue, got %T", pp)
	}
	return &ast.Incoming{X: x, Pred: p}, nil
}

// NewSelect returns a new select instruction based on the given selection
// condition type and value, and operands.
func NewSelectInst(condTyp, condVal, xTyp, xVal, yTyp, yVal interface{}) (*ast.InstSelect, error) {
	cond, err := NewValue(condTyp, condVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	x, err := NewValue(xTyp, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	y, err := NewValue(yTyp, yVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.InstSelect{Cond: cond, X: x, Y: y}, nil
}

// NewCallInst returns a new call instruction based on the given return type,
// callee name, and function arguments.
func NewCallInst(retTyp, callee, args interface{}) (*ast.InstCall, error) {
	r, ok := retTyp.(ast.Type)
	if !ok {
		return nil, errors.Errorf("invalid return type; expected ast.Type, got %T", retTyp)
	}
	cc, err := NewValue(&ast.TypeDummy{}, callee)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	c, ok := cc.(ast.NamedValue)
	if !ok {
		return nil, errors.Errorf("invalid callee type; expected ast.NamedValue, got %T", cc)
	}
	var as []ast.Value
	switch args := args.(type) {
	case []ast.Value:
		as = args
	case nil:
		// no arguments.
	default:
		return nil, errors.Errorf("invalid function arguments type; expected []ast.Value or nil, got %T", args)
	}
	return &ast.InstCall{Sig: r, Callee: c, Args: as}, nil
}

// === [ Terminators ] =========================================================

// --- [ ret ] -----------------------------------------------------------------

// NewRetTerm returns a new ret terminator based on the given return type and
// value.
func NewRetTerm(xTyp, xVal interface{}) (*ast.TermRet, error) {
	x, err := NewValue(xTyp, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &ast.TermRet{X: x}, nil
}

// --- [ br ] ------------------------------------------------------------------

// NewBrTerm returns a new unconditional br terminator based on the given target
// branch.
func NewBrTerm(targetTyp, targetVal interface{}) (*ast.TermBr, error) {
	target, err := NewValue(targetTyp, targetVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	t, ok := target.(ast.NamedValue)
	if !ok {
		return nil, errors.Errorf("invalid target branch type; expected ast.NamedValue, got %T", target)
	}
	return &ast.TermBr{Target: t}, nil
}

// --- [ conditional br ] ------------------------------------------------------

// NewCondBrTerm returns a new conditional br terminator based on the given
// branching condition type and value, and conditional target branches.
func NewCondBrTerm(condTyp, condVal, targetTrueTyp, targetTrueVal, targetFalseTyp, targetFalseVal interface{}) (*ast.TermCondBr, error) {
	cond, err := NewValue(condTyp, condVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	targetTrue, err := NewValue(targetTrueTyp, targetTrueVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	tTrue, ok := targetTrue.(ast.NamedValue)
	if !ok {
		return nil, errors.Errorf("invalid true target branch type; expected ast.NamedValue, got %T", targetTrue)
	}
	targetFalse, err := NewValue(targetFalseTyp, targetFalseVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	tFalse, ok := targetFalse.(ast.NamedValue)
	if !ok {
		return nil, errors.Errorf("invalid false target branch type; expected ast.NamedValue, got %T", targetFalse)
	}
	return &ast.TermCondBr{Cond: cond, TargetTrue: tTrue, TargetFalse: tFalse}, nil
}

// --- [ switch ] --------------------------------------------------------------

// NewSwitchTerm returns a new switch terminator based on the given control
// variable type and value, default target branch and switch cases.
func NewSwitchTerm(xTyp, xVal, targetDefaultTyp, targetDefaultVal, cases interface{}) (*ast.TermSwitch, error) {
	x, err := NewValue(xTyp, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	targetDefault, err := NewValue(targetDefaultTyp, targetDefaultVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	tDefault, ok := targetDefault.(ast.NamedValue)
	if !ok {
		return nil, errors.Errorf("invalid default target branch type; expected ast.NamedValue, got %T", targetDefault)
	}
	var cs []*ast.Case
	switch cases := cases.(type) {
	case []*ast.Case:
		cs = cases
	case nil:
		// no cases.
	default:
		return nil, errors.Errorf("invalid switch cases type; expected []*ast.Case or nil, got %T", cases)
	}
	return &ast.TermSwitch{X: x, TargetDefault: tDefault, Cases: cs}, nil
}

// NewCaseList returns a new switch case list based on the given case.
func NewCaseList(switchCase interface{}) ([]*ast.Case, error) {
	c, ok := switchCase.(*ast.Case)
	if !ok {
		return nil, errors.Errorf("invalid switch case type; expected *ast.Case, got %T", switchCase)
	}
	return []*ast.Case{c}, nil
}

// AppendCase appends the given case to the switch case list.
func AppendCase(cases, switchCase interface{}) ([]*ast.Case, error) {
	cs, ok := cases.([]*ast.Case)
	if !ok {
		return nil, errors.Errorf("invalid switch case list type; expected []*ast.Case, got %T", cases)
	}
	c, ok := switchCase.(*ast.Case)
	if !ok {
		return nil, errors.Errorf("invalid switch case type; expected *ast.Case, got %T", switchCase)
	}
	return append(cs, c), nil
}

// NewCase returns a new switch case based on the given case comparand and
// target branch.
func NewCase(xTyp, xVal, targetTyp, targetVal interface{}) (*ast.Case, error) {
	xValue, err := NewValue(xTyp, xVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	x, ok := xValue.(*ast.IntConst)
	if !ok {
		return nil, errors.Errorf("invalid case comparand type; expected *ast.IntConst, got %T", xValue)
	}
	target, err := NewValue(targetTyp, targetVal)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	t, ok := target.(ast.NamedValue)
	if !ok {
		return nil, errors.Errorf("invalid target branch type; expected ast.NamedValue, got %T", target)
	}
	return &ast.Case{X: x, Target: t}, nil
}

// ### [ Helper functions ] ####################################################

// getTokenString returns the string literal of the given token.
func getTokenString(tok interface{}) (string, error) {
	t, ok := tok.(*token.Token)
	if !ok {
		return "", errors.Errorf("invalid token type; expected *token.Token, got %T", tok)
	}
	return string(t.Lit), nil
}

// getInt64 returns the int64 representation of the given integer literal.
func getInt64(lit interface{}) (int64, error) {
	l, ok := lit.(*IntLit)
	if !ok {
		return 0, errors.Errorf("invalid array length type; expected *astx.IntLit, got %T", lit)
	}
	switch l.lit {
	case "true":
		return 1, nil
	case "false":
		return 0, nil
	}
	n, err := strconv.ParseInt(l.lit, 10, 64)
	if err != nil {
		return 0, errors.WithStack(err)
	}
	return n, nil
}
