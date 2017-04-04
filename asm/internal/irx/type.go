package irx

import (
	"fmt"

	"github.com/llir/llvm/asm/internal/ast"
	"github.com/llir/llvm/ir/types"
)

// irType returns the corresponding LLVM IR type of the given type.
func (m *Module) irType(old ast.Type) types.Type {
	switch old := old.(type) {
	case *ast.VoidType:
		return types.Void
	case *ast.FuncType:
		params := make([]*types.Param, len(old.Params))
		for i, oldParam := range old.Params {
			params[i] = types.NewParam(oldParam.Name, m.irType(oldParam.Type))
		}
		typ := types.NewFunc(m.irType(old.Ret), params...)
		typ.Variadic = old.Variadic
		return typ
	case *ast.IntType:
		return types.NewInt(old.Size)
	case *ast.FloatType:
		switch old.Kind {
		case ast.FloatKindIEEE_16:
			return types.Half
		case ast.FloatKindIEEE_32:
			return types.Float
		case ast.FloatKindIEEE_64:
			return types.Double
		case ast.FloatKindIEEE_128:
			return types.FP128
		case ast.FloatKindDoubleExtended_80:
			return types.X86_FP80
		case ast.FloatKindDoubleDouble_128:
			return types.PPC_FP128
		default:
			panic(fmt.Errorf("support for %v not yet implemented", old.Kind))
		}
	case *ast.PointerType:
		return types.NewPointer(m.irType(old.Elem))
	case *ast.VectorType:
		return types.NewVector(m.irType(old.Elem), old.Len)
	case *ast.LabelType:
		return types.Label
	case *ast.MetadataType:
		return types.Metadata
	case *ast.ArrayType:
		return types.NewArray(m.irType(old.Elem), old.Len)
	case *ast.StructType:
		fields := make([]types.Type, len(old.Fields))
		for i, oldField := range old.Fields {
			fields[i] = m.irType(oldField)
		}
		typ := types.NewStruct(fields...)
		typ.Opaque = old.Opaque
		return typ
	case *ast.NamedType:
		return m.getType(old.Name)
	case *ast.NamedTypeDummy:
		return m.getType(old.Name)
	case *ast.TypeDummy:
		panic("invalid type *ast.TypeDummy; dummy types should have been translated during parsing by astx")
	default:
		panic(fmt.Errorf("support for %T not yet implemented", old))
	}
}
