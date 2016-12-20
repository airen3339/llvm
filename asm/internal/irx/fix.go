//+build ignore

// Fix dummy values as follows.
//
// Per module.
//
//    1. Index type definitions.
//    2. Index global variables.
//    3. Index functions.
//    4. Fix body of named types.
//       - i.e. look up and set Def for each *types.NamedType.
//    6. Replace dummy instructions containing dummy Type method
//       implementations; e.g. *dummy.InstGetElementPtr.
//
// Per function.
//
//    1. Force generate local IDs for unnamed basic blocks and instructions.
//    2. Index basic blocks.
//    3. Index function parameters.
//    4. Fix dummy instructions and terminators.
//       - e.g. replace *dummy.InstCall with *ir.InstCall
//       - Replace function and label names with *ir.Function and *ir.BasicBlock
//         values.
//       - Leave dummy operands (e.g. *dummy.Local, *dummy.Global,
//         *dummy.InstPhi and *dummy.InstCall) as these will be replaced
//         in a later stage.
//    5. Index local variables produced by instructions.
//    6. Replace dummy operands of instructions and terminators.

package irx

import (
	"fmt"

	"github.com/llir/llvm/internal/dummy"
	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/irutil"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
)

// === [ Modules ] =============================================================

// fixModule replaces dummy values within the given module with their real
// values.
func fixModule(m *ir.Module) *ir.Module {

	// TODO: Remove debug output.
	//fmt.Println("=== [ globals ] ===")
	//pretty.Println(fix.globals)

	// Fix body of named types.
	visit := func(node interface{}) bool {
		old, ok := node.(*types.NamedType)
		if !ok {
			return true
		}
		if _, ok := old.Def(); !ok {
			typ := fix.getType(old.Name())
			def, ok := typ.Def()
			if !ok {
				panic(fmt.Sprintf("invalid type definition %q; expected underlying definition, got nil", typ.Name()))
			}
			old.SetDef(def)
		}
		return true
	}
	irutil.Walk(m, visit)

	// Replace dummy instructions containing dummy Type method implementations;
	// e.g. *dummy.InstGetElementPtr.
	visit = func(node interface{}) bool {
		block, ok := node.(*ir.BasicBlock)
		if !ok {
			return true
		}
		var insts []ir.Instruction
		for _, inst := range block.Insts() {
			switch old := inst.(type) {
			case *dummy.InstGetElementPtr:
				// Validate elem against old.Src().Type().Elem().
				src := old.Src()
				st, ok := src.Type().(*types.PointerType)
				if !ok {
					panic(fmt.Sprintf("invalid source type; expected *types.Pointer, got %T", src.Type()))
				}
				if !old.ElemType().Equal(st.Elem()) {
					panic(fmt.Sprintf("type mismatch between element type `%v` and source address element type `%v`", old.ElemType(), st.Elem()))
				}
				// Replace dummy getelementptr instruction with real instruction.
				new := ir.NewGetElementPtr(src, old.Indices()...)
				new.SetParent(old.Parent())
				new.SetName(old.Name())
				inst = new
			}
			insts = append(insts, inst)
		}
		block.SetInsts(insts)
		return true
	}
	irutil.Walk(m, visit)

	// Fix globals.
	for _, global := range globals {
		fix.fixGlobal(global)
	}

	// Fix functions.
	for _, f := range funcs {
		fix.fixFunction(f)
	}

	return m
}

// === [ Type definitions ] ====================================================

// === [ Global variables ] ====================================================

// fixGlobal replaces dummy values within the given global variable with their
// real values.
func (fix *fixer) fixGlobal(old *ir.Global) {
	if init, ok := old.Init(); ok {
		if init, ok := fix.fixConstant(init); ok {
			old.SetInit(init)
		}
	}
}

// === [ Functions ] ===========================================================

// fixFunction replaces dummy values within the given function with their real
// values.
func (fix *fixer) fixFunction(f *ir.Function) {
	// Reset locals.
	fix.locals = make(map[string]value.Named)

	// Force generate local IDs.
	_ = f.String()

	// Index basic blocks.
	blocks := f.Blocks()
	for _, block := range blocks {
		name := block.Name()
		if _, ok := fix.locals[name]; ok {
			panic(fmt.Sprintf("basic block label %q already present; old `%v`, new `%v`", name, fix.locals[name], block))
		}
		fix.locals[name] = block
	}

	// Index function parameters.
	for _, param := range f.Params() {
		name := param.Name()
		if _, ok := fix.locals[name]; ok {
			panic(fmt.Sprintf("function parameter name %q already present; old `%v`, new `%v`", name, fix.locals[name], param))
		}
		fix.locals[name] = param
	}

	// Fix dummy instructions and terminators.
	for _, block := range blocks {
		var insts []ir.Instruction
		for _, inst := range block.Insts() {
			switch old := inst.(type) {
			case *dummy.InstLoad:
				inst = fix.fixLoadInstDummy(old)
			case *dummy.InstPhi:
				inst = fix.fixPhiInstDummy(old)
			case *dummy.InstCall:
				inst = fix.fixCallInstDummy(old)
			}
			insts = append(insts, inst)
		}
		block.SetInsts(insts)
		term := block.Term()
		switch old := term.(type) {
		case *dummy.TermBr:
			term = fix.fixBrTermDummy(old)
		case *dummy.TermCondBr:
			term = fix.fixCondBrTermDummy(old)
		case *dummy.TermSwitch:
			term = fix.fixSwitchTermDummy(old)
		}
		block.SetTerm(term)
	}

	// Index local variables produced by instructions.
	for _, block := range blocks {
		for _, inst := range block.Insts() {
			if inst, ok := inst.(value.Named); ok {
				name := inst.Name()
				if _, ok := fix.locals[name]; ok {
					panic(fmt.Sprintf("instruction name %q already present; old `%v`, new `%v`", name, fix.locals[name], inst))
				}
				fix.locals[name] = inst
			}
		}
	}

	// TODO: Remove debug output.
	//fmt.Printf("=== [ locals of %q ] ===\n", f.Name())
	//pretty.Println(fix.locals)

	// Fix basic blocks.
	for _, block := range blocks {
		fix.fixBlock(block)
	}
}

// === [ Values ] ==============================================================

// fixValue replaces given dummy value with its real value. The boolean return
// value indicates if a dummy value was replaced.
func (fix *fixer) fixValue(old value.Value) (value.Value, bool) {
	// TODO: Add all instructions producing values.
	switch old := old.(type) {
	case constant.Constant:
		return fix.fixConstant(old)
	case *dummy.Local:
		return fix.getLocal(old.Name()), true
	case *ir.InstAdd, *ir.InstFAdd, *ir.InstSub, *ir.InstFSub, *ir.InstMul, *ir.InstFMul, *ir.InstUDiv, *ir.InstSDiv, *ir.InstFDiv, *ir.InstURem, *ir.InstSRem, *ir.InstFRem:
		// nothing to do; valid value.
	case *ir.InstICmp:
		// nothing to do; valid value.
	default:
		panic(fmt.Sprintf("support for value type %T not yet implemented", old))
	}
	return old, false
}

// === [ Constants ] ===========================================================

// fixConstant replaces given dummy value with its real value. The boolean
// return value indicates if a dummy value was replaced.
func (fix *fixer) fixConstant(old constant.Constant) (constant.Constant, bool) {
	// TODO: Add all constant expressions.
	switch old := old.(type) {
	case *dummy.Global:
		global := fix.getGlobal(old.Name())
		g, ok := global.(constant.Constant)
		if !ok {
			panic(fmt.Sprintf("invalid global type; expected constant.Constant, got %T", global))
		}
		return g, true
	case *constant.Int:
		// nothing to do; valid value.
	case *constant.Float:
		// nothing to do; valid value.
	case *constant.Null:
		// nothing to do; valid value.
	case *constant.Vector:
		// nothing to do; valid value.
	case *constant.Array:
		// nothing to do; valid value.
	case *constant.ExprGetElementPtr:
		return fix.fixGetElementPtrExpr(old), true
	default:
		panic(fmt.Sprintf("support for constant type %T not yet implemented", old))
	}
	return old, false
}

// --- [ Memory expressions ] --------------------------------------------------

// fixGetElementPtrExpr replaces dummy values within the given getelementptr
// expression with their real values.
func (fix *fixer) fixGetElementPtrExpr(old *constant.ExprGetElementPtr) *constant.ExprGetElementPtr {
	if src, ok := fix.fixConstant(old.Src()); ok {
		old.SetSrc(src)
	}
	var indices []constant.Constant
	for _, index := range old.Indices() {
		index, _ = fix.fixConstant(index)
		indices = append(indices, index)
	}
	old.SetIndices(indices)
	return old
}

// === [ Basic blocks ] ========================================================

// fixBlock replaces dummy values within the given basic block with their real
// values.
func (fix *fixer) fixBlock(block *ir.BasicBlock) {
	// Fix instructions.
	var insts []ir.Instruction
	for _, old := range block.Insts() {
		inst := fix.fixInst(old)
		insts = append(insts, inst)
	}
	block.SetInsts(insts)

	// Fix terminator.
	term := fix.fixTerm(block.Term())
	block.SetTerm(term)
}

// === [ Instructions ] ========================================================

// fixInst replaces dummy values within the given instruction with their real
// values.
func (fix *fixer) fixInst(inst ir.Instruction) ir.Instruction {
	switch inst := inst.(type) {
	// Binary instructions
	case *ir.InstAdd:
		return fix.fixAddInst(inst)
	case *ir.InstFAdd:
		return fix.fixFAddInst(inst)
	case *ir.InstSub:
		return fix.fixSubInst(inst)
	case *ir.InstFSub:
		return fix.fixFSubInst(inst)
	case *ir.InstMul:
		return fix.fixMulInst(inst)
	case *ir.InstFMul:
		return fix.fixFMulInst(inst)
	case *ir.InstUDiv:
		return fix.fixUDivInst(inst)
	case *ir.InstSDiv:
		return fix.fixSDivInst(inst)
	case *ir.InstFDiv:
		return fix.fixFDivInst(inst)
	case *ir.InstURem:
		return fix.fixURemInst(inst)
	case *ir.InstSRem:
		return fix.fixSRemInst(inst)
	case *ir.InstFRem:
		return fix.fixFRemInst(inst)
	// Bitwise instructions
	case *ir.InstShl:
		return fix.fixShlInst(inst)
	case *ir.InstLShr:
		return fix.fixLShrInst(inst)
	case *ir.InstAShr:
		return fix.fixAShrInst(inst)
	case *ir.InstAnd:
		return fix.fixAndInst(inst)
	case *ir.InstOr:
		return fix.fixOrInst(inst)
	case *ir.InstXor:
		return fix.fixXorInst(inst)
	// Memory instructions
	case *ir.InstAlloca:
		return fix.fixAllocaInst(inst)
	case *ir.InstLoad:
		return fix.fixLoadInst(inst)
	case *ir.InstStore:
		return fix.fixStoreInst(inst)
	case *ir.InstGetElementPtr:
		return fix.fixGetElementPtrInst(inst)
	// Conversion instructions
	case *ir.InstTrunc:
		return fix.fixTruncInst(inst)
	case *ir.InstZExt:
		return fix.fixZExtInst(inst)
	case *ir.InstSExt:
		return fix.fixSExtInst(inst)
	case *ir.InstFPTrunc:
		return fix.fixFPTruncInst(inst)
	case *ir.InstFPExt:
		return fix.fixFPExtInst(inst)
	case *ir.InstFPToUI:
		return fix.fixFPToUIInst(inst)
	case *ir.InstFPToSI:
		return fix.fixFPToSIInst(inst)
	case *ir.InstUIToFP:
		return fix.fixUIToFPInst(inst)
	case *ir.InstSIToFP:
		return fix.fixSIToFPInst(inst)
	case *ir.InstPtrToInt:
		return fix.fixPtrToIntInst(inst)
	case *ir.InstIntToPtr:
		return fix.fixIntToPtrInst(inst)
	case *ir.InstBitCast:
		return fix.fixBitCastInst(inst)
	case *ir.InstAddrSpaceCast:
		return fix.fixAddrSpaceCastInst(inst)
	// Other instructions
	case *ir.InstICmp:
		return fix.fixICmpInst(inst)
	case *ir.InstFCmp:
		return fix.fixFCmpInst(inst)
	case *ir.InstPhi:
		return fix.fixPhiInst(inst)
	case *ir.InstSelect:
		return fix.fixSelectInst(inst)
	case *ir.InstCall:
		return fix.fixCallInst(inst)
	default:
		panic(fmt.Sprintf("support for instruction type %T not yet implemented", inst))
	}
}

// --- [ Binary instructions ] -------------------------------------------------

// fixAddInst replaces dummy values within the given add instruction with their
// real values.
func (fix *fixer) fixAddInst(old *ir.InstAdd) *ir.InstAdd {
	if x, ok := fix.fixValue(old.X()); ok {
		old.SetX(x)
	}
	if y, ok := fix.fixValue(old.Y()); ok {
		old.SetY(y)
	}
	return old
}

// fixFAddInst replaces dummy values within the given fadd instruction with their
// real values.
func (fix *fixer) fixFAddInst(old *ir.InstFAdd) *ir.InstFAdd {
	if x, ok := fix.fixValue(old.X()); ok {
		old.SetX(x)
	}
	if y, ok := fix.fixValue(old.Y()); ok {
		old.SetY(y)
	}
	return old
}

// fixSubInst replaces dummy values within the given sub instruction with their
// real values.
func (fix *fixer) fixSubInst(old *ir.InstSub) *ir.InstSub {
	if x, ok := fix.fixValue(old.X()); ok {
		old.SetX(x)
	}
	if y, ok := fix.fixValue(old.Y()); ok {
		old.SetY(y)
	}
	return old
}

// fixFSubInst replaces dummy values within the given fsub instruction with their
// real values.
func (fix *fixer) fixFSubInst(old *ir.InstFSub) *ir.InstFSub {
	if x, ok := fix.fixValue(old.X()); ok {
		old.SetX(x)
	}
	if y, ok := fix.fixValue(old.Y()); ok {
		old.SetY(y)
	}
	return old
}

// fixMulInst replaces dummy values within the given mul instruction with their
// real values.
func (fix *fixer) fixMulInst(old *ir.InstMul) *ir.InstMul {
	if x, ok := fix.fixValue(old.X()); ok {
		old.SetX(x)
	}
	if y, ok := fix.fixValue(old.Y()); ok {
		old.SetY(y)
	}
	return old
}

// fixFMulInst replaces dummy values within the given fmul instruction with their
// real values.
func (fix *fixer) fixFMulInst(old *ir.InstFMul) *ir.InstFMul {
	if x, ok := fix.fixValue(old.X()); ok {
		old.SetX(x)
	}
	if y, ok := fix.fixValue(old.Y()); ok {
		old.SetY(y)
	}
	return old
}

// fixUDivInst replaces dummy values within the given udiv instruction with their
// real values.
func (fix *fixer) fixUDivInst(old *ir.InstUDiv) *ir.InstUDiv {
	if x, ok := fix.fixValue(old.X()); ok {
		old.SetX(x)
	}
	if y, ok := fix.fixValue(old.Y()); ok {
		old.SetY(y)
	}
	return old
}

// fixSDivInst replaces dummy values within the given sdiv instruction with their
// real values.
func (fix *fixer) fixSDivInst(old *ir.InstSDiv) *ir.InstSDiv {
	if x, ok := fix.fixValue(old.X()); ok {
		old.SetX(x)
	}
	if y, ok := fix.fixValue(old.Y()); ok {
		old.SetY(y)
	}
	return old
}

// fixFDivInst replaces dummy values within the given fdiv instruction with their
// real values.
func (fix *fixer) fixFDivInst(old *ir.InstFDiv) *ir.InstFDiv {
	if x, ok := fix.fixValue(old.X()); ok {
		old.SetX(x)
	}
	if y, ok := fix.fixValue(old.Y()); ok {
		old.SetY(y)
	}
	return old
}

// fixURemInst replaces dummy values within the given urem instruction with their
// real values.
func (fix *fixer) fixURemInst(old *ir.InstURem) *ir.InstURem {
	if x, ok := fix.fixValue(old.X()); ok {
		old.SetX(x)
	}
	if y, ok := fix.fixValue(old.Y()); ok {
		old.SetY(y)
	}
	return old
}

// fixSRemInst replaces dummy values within the given srem instruction with their
// real values.
func (fix *fixer) fixSRemInst(old *ir.InstSRem) *ir.InstSRem {
	if x, ok := fix.fixValue(old.X()); ok {
		old.SetX(x)
	}
	if y, ok := fix.fixValue(old.Y()); ok {
		old.SetY(y)
	}
	return old
}

// fixFRemInst replaces dummy values within the given frem instruction with their
// real values.
func (fix *fixer) fixFRemInst(old *ir.InstFRem) *ir.InstFRem {
	if x, ok := fix.fixValue(old.X()); ok {
		old.SetX(x)
	}
	if y, ok := fix.fixValue(old.Y()); ok {
		old.SetY(y)
	}
	return old
}

// --- [ Bitwise instructions ] ------------------------------------------------

// fixShlInst replaces dummy values within the given shl instruction with their
// real values.
func (fix *fixer) fixShlInst(old *ir.InstShl) *ir.InstShl {
	if x, ok := fix.fixValue(old.X()); ok {
		old.SetX(x)
	}
	if y, ok := fix.fixValue(old.Y()); ok {
		old.SetY(y)
	}
	return old
}

// fixLShrInst replaces dummy values within the given lshr instruction with their
// real values.
func (fix *fixer) fixLShrInst(old *ir.InstLShr) *ir.InstLShr {
	if x, ok := fix.fixValue(old.X()); ok {
		old.SetX(x)
	}
	if y, ok := fix.fixValue(old.Y()); ok {
		old.SetY(y)
	}
	return old
}

// fixAShrInst replaces dummy values within the given ashr instruction with their
// real values.
func (fix *fixer) fixAShrInst(old *ir.InstAShr) *ir.InstAShr {
	if x, ok := fix.fixValue(old.X()); ok {
		old.SetX(x)
	}
	if y, ok := fix.fixValue(old.Y()); ok {
		old.SetY(y)
	}
	return old
}

// fixAndInst replaces dummy values within the given and instruction with their
// real values.
func (fix *fixer) fixAndInst(old *ir.InstAnd) *ir.InstAnd {
	if x, ok := fix.fixValue(old.X()); ok {
		old.SetX(x)
	}
	if y, ok := fix.fixValue(old.Y()); ok {
		old.SetY(y)
	}
	return old
}

// fixOrInst replaces dummy values within the given or instruction with their
// real values.
func (fix *fixer) fixOrInst(old *ir.InstOr) *ir.InstOr {
	if x, ok := fix.fixValue(old.X()); ok {
		old.SetX(x)
	}
	if y, ok := fix.fixValue(old.Y()); ok {
		old.SetY(y)
	}
	return old
}

// fixXorInst replaces dummy values within the given xor instruction with their
// real values.
func (fix *fixer) fixXorInst(old *ir.InstXor) *ir.InstXor {
	if x, ok := fix.fixValue(old.X()); ok {
		old.SetX(x)
	}
	if y, ok := fix.fixValue(old.Y()); ok {
		old.SetY(y)
	}
	return old
}

// --- [ Memory instructions ] -------------------------------------------------

// fixAllocaInst replaces dummy values within the given alloca instruction with
// their real values.
func (fix *fixer) fixAllocaInst(old *ir.InstAlloca) *ir.InstAlloca {
	if nelems, ok := old.NElems(); ok {
		if nelems, ok := fix.fixValue(nelems); ok {
			old.SetNElems(nelems)
		}
	}
	return old
}

// fixLoadInst replaces dummy values within the given load instruction with
// their real values.
func (fix *fixer) fixLoadInst(old *ir.InstLoad) *ir.InstLoad {
	if src, ok := fix.fixValue(old.Src()); ok {
		old.SetSrc(src)
	}
	return old
}

// fixLoadInstDummy replaces dummy values within the given load instruction with
// their real values.
func (fix *fixer) fixLoadInstDummy(old *dummy.InstLoad) *ir.InstLoad {
	// Validate elem against old.Type().
	if !old.ElemType().Equal(old.Type()) {
		panic(fmt.Sprintf("type mismatch between element type `%v` and source address element type `%v`", old.ElemType(), old.Type()))
	}
	// Replace dummy load instruction with real instruction.
	inst := ir.NewLoad(old.Src())
	inst.SetParent(old.Parent())
	inst.SetName(old.Name())
	return inst
}

// fixStoreInst replaces dummy values within the given store instruction with
// their real values.
func (fix *fixer) fixStoreInst(old *ir.InstStore) *ir.InstStore {
	if src, ok := fix.fixValue(old.Src()); ok {
		old.SetSrc(src)
	}
	if dst, ok := fix.fixValue(old.Dst()); ok {
		old.SetDst(dst)
	}
	return old
}

// fixGetElementPtrInst replaces dummy values within the given getelementptr
// instruction with their real values.
func (fix *fixer) fixGetElementPtrInst(old *ir.InstGetElementPtr) *ir.InstGetElementPtr {
	if src, ok := fix.fixValue(old.Src()); ok {
		old.SetSrc(src)
	}
	var indices []value.Value
	for _, index := range old.Indices() {
		index, _ = fix.fixValue(index)
		indices = append(indices, index)
	}
	old.SetIndices(indices)
	return old
}

// --- [ Conversion instructions ] ---------------------------------------------

// fixTruncInst replaces dummy values within the given trunc instruction with
// their real values.
func (fix *fixer) fixTruncInst(old *ir.InstTrunc) *ir.InstTrunc {
	if from, ok := fix.fixValue(old.From()); ok {
		old.SetFrom(from)
	}
	return old
}

// fixZExtInst replaces dummy values within the given zext instruction with
// their real values.
func (fix *fixer) fixZExtInst(old *ir.InstZExt) *ir.InstZExt {
	if from, ok := fix.fixValue(old.From()); ok {
		old.SetFrom(from)
	}
	return old
}

// fixSExtInst replaces dummy values within the given sext instruction with
// their real values.
func (fix *fixer) fixSExtInst(old *ir.InstSExt) *ir.InstSExt {
	if from, ok := fix.fixValue(old.From()); ok {
		old.SetFrom(from)
	}
	return old
}

// fixFPTruncInst replaces dummy values within the given fptrunc instruction
// with their real values.
func (fix *fixer) fixFPTruncInst(old *ir.InstFPTrunc) *ir.InstFPTrunc {
	if from, ok := fix.fixValue(old.From()); ok {
		old.SetFrom(from)
	}
	return old
}

// fixFPExtInst replaces dummy values within the given fpext instruction with
// their real values.
func (fix *fixer) fixFPExtInst(old *ir.InstFPExt) *ir.InstFPExt {
	if from, ok := fix.fixValue(old.From()); ok {
		old.SetFrom(from)
	}
	return old
}

// fixFPToUIInst replaces dummy values within the given fptoui instruction with
// their real values.
func (fix *fixer) fixFPToUIInst(old *ir.InstFPToUI) *ir.InstFPToUI {
	if from, ok := fix.fixValue(old.From()); ok {
		old.SetFrom(from)
	}
	return old
}

// fixFPToSIInst replaces dummy values within the given fptosi instruction with
// their real values.
func (fix *fixer) fixFPToSIInst(old *ir.InstFPToSI) *ir.InstFPToSI {
	if from, ok := fix.fixValue(old.From()); ok {
		old.SetFrom(from)
	}
	return old
}

// fixUIToFPInst replaces dummy values within the given uitofp instruction with
// their real values.
func (fix *fixer) fixUIToFPInst(old *ir.InstUIToFP) *ir.InstUIToFP {
	if from, ok := fix.fixValue(old.From()); ok {
		old.SetFrom(from)
	}
	return old
}

// fixSIToFPInst replaces dummy values within the given sitofp instruction with
// their real values.
func (fix *fixer) fixSIToFPInst(old *ir.InstSIToFP) *ir.InstSIToFP {
	if from, ok := fix.fixValue(old.From()); ok {
		old.SetFrom(from)
	}
	return old
}

// fixPtrToIntInst replaces dummy values within the given ptrtoint instruction
// with their real values.
func (fix *fixer) fixPtrToIntInst(old *ir.InstPtrToInt) *ir.InstPtrToInt {
	if from, ok := fix.fixValue(old.From()); ok {
		old.SetFrom(from)
	}
	return old
}

// fixIntToPtrInst replaces dummy values within the given inttoptr instruction
// with their real values.
func (fix *fixer) fixIntToPtrInst(old *ir.InstIntToPtr) *ir.InstIntToPtr {
	if from, ok := fix.fixValue(old.From()); ok {
		old.SetFrom(from)
	}
	return old
}

// fixBitCastInst replaces dummy values within the given bitcast instruction
// with their real values.
func (fix *fixer) fixBitCastInst(old *ir.InstBitCast) *ir.InstBitCast {
	if from, ok := fix.fixValue(old.From()); ok {
		old.SetFrom(from)
	}
	return old
}

// fixAddrSpaceCastInst replaces dummy values within the given addrspacecast
// instruction with their real values.
func (fix *fixer) fixAddrSpaceCastInst(old *ir.InstAddrSpaceCast) *ir.InstAddrSpaceCast {
	if from, ok := fix.fixValue(old.From()); ok {
		old.SetFrom(from)
	}
	return old
}

// --- [ Other instructions ] --------------------------------------------------

// fixICmpInst replaces dummy values within the given icmp instruction with
// their real values.
func (fix *fixer) fixICmpInst(old *ir.InstICmp) *ir.InstICmp {
	if x, ok := fix.fixValue(old.X()); ok {
		old.SetX(x)
	}
	if y, ok := fix.fixValue(old.Y()); ok {
		old.SetY(y)
	}
	return old
}

// fixFCmpInst replaces dummy values within the given fcmp instruction with
// their real values.
func (fix *fixer) fixFCmpInst(old *ir.InstFCmp) *ir.InstFCmp {
	if x, ok := fix.fixValue(old.X()); ok {
		old.SetX(x)
	}
	if y, ok := fix.fixValue(old.Y()); ok {
		old.SetY(y)
	}
	return old
}

// fixPhiInstDummy replaces the given dummy phi instruction with a real phi
// instruction, and replaces dummy the instruction with their real values.
func (fix *fixer) fixPhiInstDummy(old *dummy.InstPhi) *ir.InstPhi {
	var incs []*ir.Incoming
	for _, inc := range old.Incs() {
		pred := fix.getBlock(inc.Pred())
		// Leave inc.x unchanged for now. It may contain dummy values. fixPhiInst
		// will replace these later.
		//
		// We cannot replace them yet, as all local variables have not been
		// indexed yet, as the time of the call to fixPhiInstDummy.
		x, ok := inc.X().(value.Value)
		if !ok {
			panic(fmt.Sprintf("invalid x type; expected value.Value, got %T", inc.X()))
		}
		incs = append(incs, ir.NewIncoming(x, pred))
	}
	inst := ir.NewPhi(incs...)
	inst.SetParent(old.Parent())
	inst.SetName(old.Name())
	return inst
}

// fixPhiInst replaces dummy values within the given phi instruction with their
// real values.
func (fix *fixer) fixPhiInst(old *ir.InstPhi) *ir.InstPhi {
	for _, inc := range old.Incs() {
		if x, ok := fix.fixValue(inc.X()); ok {
			inc.SetX(x)
		}
	}
	return old
}

// fixSelectInst replaces dummy values within the given select instruction with
// their real values.
func (fix *fixer) fixSelectInst(old *ir.InstSelect) *ir.InstSelect {
	if cond, ok := fix.fixValue(old.Cond()); ok {
		old.SetCond(cond)
	}
	if x, ok := fix.fixValue(old.X()); ok {
		old.SetX(x)
	}
	if y, ok := fix.fixValue(old.Y()); ok {
		old.SetY(y)
	}
	return old
}

// fixCallInst replaces dummy values within the given call instruction with
// their real values.
func (fix *fixer) fixCallInst(old *ir.InstCall) *ir.InstCall {
	var args []value.Value
	for _, arg := range old.Args() {
		arg, _ = fix.fixValue(arg)
		args = append(args, arg)
	}
	old.SetArgs(args)
	return old
}

// fixCallInstDummy replaces the given dummy call instruction with a real call
// instruction, and replaces dummy the instruction with their real values.
func (fix *fixer) fixCallInstDummy(old *dummy.InstCall) *ir.InstCall {
	var callee value.Named
	if old.CalleeLocal() {
		callee = fix.getLocal(old.Callee())
	} else {
		callee = fix.getGlobal(old.Callee())
		if callee, ok := callee.(*ir.Function); ok {
			if got, want := old.Type(), callee.RetType(); !got.Equal(want) {
				panic(fmt.Sprintf("return type mismatch; expected `%v`, got `%v`", want, got))
			}
		}
	}
	// Leave args unchanged for now. It may contain dummy values. fixCallInst
	// will replace these later.
	//
	// We cannot replace them yet, as all local variables have not been indexed
	// yet, as the time of the call to fixCallInstDummy.
	inst := ir.NewCall(callee, old.Args()...)
	inst.SetParent(old.Parent())
	inst.SetName(old.Name())
	return inst
}

// === [ Terminators ] =========================================================

// fixTerm replaces dummy values within the given terminator with their real
// values.
func (fix *fixer) fixTerm(term ir.Terminator) ir.Terminator {
	switch term := term.(type) {
	case *ir.TermRet:
		return fix.fixRetTerm(term)
	case *ir.TermBr:
		// nothing to do; contains no values.
		return term
	case *ir.TermCondBr:
		return fix.fixCondBrTerm(term)
	case *ir.TermSwitch:
		return fix.fixSwitchTerm(term)
	case *ir.TermUnreachable:
		// nothing to do; contains no values.
		return term
	default:
		panic(fmt.Sprintf("support for terminator type %T not yet implemented", term))
	}
}

// fixRetTerm replaces dummy values within the given ret terminator with their
// real values.
func (fix *fixer) fixRetTerm(old *ir.TermRet) *ir.TermRet {
	if x, ok := old.X(); ok {
		if x, ok := fix.fixValue(x); ok {
			old.SetX(x)
		}
	}
	return old
}

// fixBrTermDummy replaces the given dummy unconditional br terminator with a
// real unconditional br terminator, and replaces dummy values within the
// terminator with their real values.
func (fix *fixer) fixBrTermDummy(old *dummy.TermBr) *ir.TermBr {
	target := fix.getBlock(old.Target())
	term := ir.NewBr(target)
	term.SetParent(old.Parent())
	return term
}

// fixCondBrTerm replaces dummy values within the given conditional br
// terminator with their real values.
func (fix *fixer) fixCondBrTerm(old *ir.TermCondBr) *ir.TermCondBr {
	if cond, ok := fix.fixValue(old.Cond()); ok {
		old.SetCond(cond)
	}
	return old
}

// fixSwitchTerm replaces dummy values within the given switch terminator with
// their real values.
func (fix *fixer) fixSwitchTerm(old *ir.TermSwitch) *ir.TermSwitch {
	if x, ok := fix.fixValue(old.X()); ok {
		old.SetX(x)
	}
	for _, c := range old.Cases() {
		if xValue, ok := fix.fixValue(c.X()); ok {
			x, ok := xValue.(*constant.Int)
			if !ok {
				panic(fmt.Sprintf("invalid x type; expected *constant.Int, got %T", xValue))
			}
			c.SetX(x)
		}
	}
	return old
}

// fixCondBrTermDummy replaces the given dummy conditional br terminator with a
// real conditional br terminator, and replaces dummy values within the
// terminator with their real values.
func (fix *fixer) fixCondBrTermDummy(old *dummy.TermCondBr) *ir.TermCondBr {
	targetTrue := fix.getBlock(old.TargetTrue())
	targetFalse := fix.getBlock(old.TargetFalse())
	// Leave old.cond unchanged for now. It may contain dummy values.
	// fixCondBrTerm will replace these later.
	//
	// We cannot replace them yet, as all local variables have not been indexed
	// yet, as the time of the call to fixCondBrTermDummy.
	term := ir.NewCondBr(old.Cond(), targetTrue, targetFalse)
	term.SetParent(old.Parent())
	return term
}

// fixSwitchTermDummy replaces the given dummy switch terminator with a real
// switch terminator, and replaces dummy values within the terminator with their
// real values.
func (fix *fixer) fixSwitchTermDummy(old *dummy.TermSwitch) *ir.TermSwitch {
	targetDefault := fix.getBlock(old.TargetDefault())
	// Leave old.x and c.x unchanged for now. They may contain dummy values.
	// fixSwitchTerm will replace these later.
	//
	// We cannot replace them yet, as all local variables have not been indexed
	// yet, as the time of the call to fixSwitchTermDummy.
	var cases []*ir.Case
	for _, c := range old.Cases() {
		target := fix.getBlock(c.Target())
		cases = append(cases, ir.NewCase(c.X(), target))
	}
	term := ir.NewSwitch(old.X(), targetDefault, cases...)
	term.SetParent(old.Parent())
	return term
}
