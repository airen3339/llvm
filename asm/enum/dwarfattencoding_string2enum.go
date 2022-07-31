// Code generated by "string2enum -linecomment -type DwarfAttEncoding ../../ir/enum"; DO NOT EDIT.

package enum

import (
	"fmt"

	"github.com/llir/llvm/ir/enum"
)

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the string2enum command to generate them again.
	var x [1]struct{}
	_ = x[enum.DwarfAttEncodingAddress-1]
	_ = x[enum.DwarfAttEncodingBoolean-2]
	_ = x[enum.DwarfAttEncodingComplexFloat-3]
	_ = x[enum.DwarfAttEncodingFloat-4]
	_ = x[enum.DwarfAttEncodingSigned-5]
	_ = x[enum.DwarfAttEncodingSignedChar-6]
	_ = x[enum.DwarfAttEncodingUnsigned-7]
	_ = x[enum.DwarfAttEncodingUnsignedChar-8]
	_ = x[enum.DwarfAttEncodingImaginaryFloat-9]
	_ = x[enum.DwarfAttEncodingPackedDecimal-10]
	_ = x[enum.DwarfAttEncodingNumericString-11]
	_ = x[enum.DwarfAttEncodingEdited-12]
	_ = x[enum.DwarfAttEncodingSignedFixed-13]
	_ = x[enum.DwarfAttEncodingUnsignedFixed-14]
	_ = x[enum.DwarfAttEncodingDecimalFloat-15]
	_ = x[enum.DwarfAttEncodingUTF-16]
	_ = x[enum.DwarfAttEncodingUCS-17]
	_ = x[enum.DwarfAttEncodingASCII-18]
}

const _DwarfAttEncoding_name = "DW_ATE_addressDW_ATE_booleanDW_ATE_complex_floatDW_ATE_floatDW_ATE_signedDW_ATE_signed_charDW_ATE_unsignedDW_ATE_unsigned_charDW_ATE_imaginary_floatDW_ATE_packed_decimalDW_ATE_numeric_stringDW_ATE_editedDW_ATE_signed_fixedDW_ATE_unsigned_fixedDW_ATE_decimal_floatDW_ATE_UTFDW_ATE_UCSDW_ATE_ASCII"

var _DwarfAttEncoding_index = [...]uint16{0, 14, 28, 48, 60, 73, 91, 106, 126, 148, 169, 190, 203, 222, 243, 263, 273, 283, 295}

// DwarfAttEncodingFromString returns the DwarfAttEncoding enum corresponding to s.
func DwarfAttEncodingFromString(s string) enum.DwarfAttEncoding {
	if len(s) == 0 {
		return 0
	}
	for i := range _DwarfAttEncoding_index[:len(_DwarfAttEncoding_index)-1] {
		if s == _DwarfAttEncoding_name[_DwarfAttEncoding_index[i]:_DwarfAttEncoding_index[i+1]] {
			return enum.DwarfAttEncoding(i + 1)
		}
	}
	panic(fmt.Errorf("unable to locate DwarfAttEncoding enum corresponding to %q", s))
}

func _(s string) {
	// Check for duplicate string values in type "DwarfAttEncoding".
	switch s {
	// 1
	case "DW_ATE_address":
	// 2
	case "DW_ATE_boolean":
	// 3
	case "DW_ATE_complex_float":
	// 4
	case "DW_ATE_float":
	// 5
	case "DW_ATE_signed":
	// 6
	case "DW_ATE_signed_char":
	// 7
	case "DW_ATE_unsigned":
	// 8
	case "DW_ATE_unsigned_char":
	// 9
	case "DW_ATE_imaginary_float":
	// 10
	case "DW_ATE_packed_decimal":
	// 11
	case "DW_ATE_numeric_string":
	// 12
	case "DW_ATE_edited":
	// 13
	case "DW_ATE_signed_fixed":
	// 14
	case "DW_ATE_unsigned_fixed":
	// 15
	case "DW_ATE_decimal_float":
	// 16
	case "DW_ATE_UTF":
	// 17
	case "DW_ATE_UCS":
	// 18
	case "DW_ATE_ASCII":
	}
}
