// Code generated by "stringer -linecomment -type DwarfCC"; DO NOT EDIT.

package enum

import "strconv"

const (
	_DwarfCC_name_0 = "DW_CC_normalDW_CC_programDW_CC_nocallDW_CC_pass_by_referenceDW_CC_pass_by_value"
	_DwarfCC_name_1 = "DW_CC_GNU_borland_fastcall_i386"
	_DwarfCC_name_2 = "DW_CC_BORLAND_safecallDW_CC_BORLAND_stdcallDW_CC_BORLAND_pascalDW_CC_BORLAND_msfastcallDW_CC_BORLAND_msreturnDW_CC_BORLAND_thiscallDW_CC_BORLAND_fastcall"
	_DwarfCC_name_3 = "DW_CC_LLVM_vectorcall"
)

var (
	_DwarfCC_index_0 = [...]uint8{0, 12, 25, 37, 60, 79}
	_DwarfCC_index_2 = [...]uint8{0, 22, 43, 63, 87, 109, 131, 153}
)

func (i DwarfCC) String() string {
	switch {
	case 1 <= i && i <= 5:
		i -= 1
		return _DwarfCC_name_0[_DwarfCC_index_0[i]:_DwarfCC_index_0[i+1]]
	case i == 65:
		return _DwarfCC_name_1
	case 176 <= i && i <= 182:
		i -= 176
		return _DwarfCC_name_2[_DwarfCC_index_2[i]:_DwarfCC_index_2[i+1]]
	case i == 192:
		return _DwarfCC_name_3
	default:
		return "DwarfCC(" + strconv.FormatInt(int64(i), 10) + ")"
	}
}
