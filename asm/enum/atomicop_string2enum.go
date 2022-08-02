// Code generated by "string2enum -linecomment -type AtomicOp ../../ir/enum"; DO NOT EDIT.

package enum

import (
	"fmt"

	"github.com/llir/llvm/ir/enum"
)

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the string2enum command to generate them again.
	var x [1]struct{}
	_ = x[enum.AtomicOpAdd-1]
	_ = x[enum.AtomicOpAnd-2]
	_ = x[enum.AtomicOpFAdd-3]
	_ = x[enum.AtomicOpFMax-4]
	_ = x[enum.AtomicOpFMin-5]
	_ = x[enum.AtomicOpFSub-6]
	_ = x[enum.AtomicOpMax-7]
	_ = x[enum.AtomicOpMin-8]
	_ = x[enum.AtomicOpNAnd-9]
	_ = x[enum.AtomicOpOr-10]
	_ = x[enum.AtomicOpSub-11]
	_ = x[enum.AtomicOpUMax-12]
	_ = x[enum.AtomicOpUMin-13]
	_ = x[enum.AtomicOpXChg-14]
	_ = x[enum.AtomicOpXor-15]
}

const _AtomicOp_name = "addandfaddfmaxfminfsubmaxminnandorsubumaxuminxchgxor"

var _AtomicOp_index = [...]uint8{0, 3, 6, 10, 14, 18, 22, 25, 28, 32, 34, 37, 41, 45, 49, 52}

// AtomicOpFromString returns the AtomicOp enum corresponding to s.
func AtomicOpFromString(s string) enum.AtomicOp {
	if len(s) == 0 {
		return 0
	}
	for i := range _AtomicOp_index[:len(_AtomicOp_index)-1] {
		if s == _AtomicOp_name[_AtomicOp_index[i]:_AtomicOp_index[i+1]] {
			return enum.AtomicOp(i + 1)
		}
	}
	panic(fmt.Errorf("unable to locate AtomicOp enum corresponding to %q", s))
}

func _(s string) {
	// Check for duplicate string values in type "AtomicOp".
	switch s {
	// 1
	case "add":
	// 2
	case "and":
	// 3
	case "fadd":
	// 4
	case "fmax":
	// 5
	case "fmin":
	// 6
	case "fsub":
	// 7
	case "max":
	// 8
	case "min":
	// 9
	case "nand":
	// 10
	case "or":
	// 11
	case "sub":
	// 12
	case "umax":
	// 13
	case "umin":
	// 14
	case "xchg":
	// 15
	case "xor":
	}
}
