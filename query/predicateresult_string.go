// generated by stringer -type=PredicateResult; DO NOT EDIT

package query

import "fmt"

const _PredicateResult_name = "PredicateResultPositivePredicateResultNegativePredicateResultUncertain"

var _PredicateResult_index = [...]uint8{0, 23, 46, 70}

func (i PredicateResult) String() string {
	if i+1 >= PredicateResult(len(_PredicateResult_index)) {
		return fmt.Sprintf("PredicateResult(%d)", i)
	}
	return _PredicateResult_name[_PredicateResult_index[i]:_PredicateResult_index[i+1]]
}
