package livecode

import (
	"github.com/pancakeswya/ot"
	"unicode/utf8"
)

func transformIndex(operation *ot.Sequence, position uint32) uint32 {
	index := int32(position)
	newIndex := index
	for _, op := range operation.Ops {
		switch opVal := op.(type) {
		case ot.Retain:
			index -= int32(opVal.N)
		case ot.Insert:
			newIndex += int32(utf8.RuneCountInString(opVal.Str))
		case ot.Delete:
			n := int32(opVal.N)
			if index < n {
				newIndex -= index
			} else {
				newIndex -= n
			}
			index += n
		}
		if index < 0 {
			break
		}
	}
	return uint32(newIndex)
}
