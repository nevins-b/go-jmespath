package jmespath

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

type JPFunction func(arguments []interface{}) (interface{}, error)

type JPType string

const (
	JPUnknown     JPType = "unknown"
	JPNumber      JPType = "number"
	JPString      JPType = "string"
	JPArray       JPType = "array"
	JPObject      JPType = "object"
	JPArrayNumber JPType = "array[number]"
	JPArrayString JPType = "array[string]"
	JPExpref      JPType = "expref"
	JPAny         JPType = "any"
)

type FunctionEntry struct {
	name      string
	arguments []ArgSpec
	handler   JPFunction
	hasExpRef bool
}

type ArgSpec struct {
	types    []JPType
	variadic bool
}

type byExprString struct {
	intr     *treeInterpreter
	node     ASTNode
	items    []interface{}
	hasError bool
}

func (a *byExprString) Len() int {
	return len(a.items)
}
func (a *byExprString) Swap(i, j int) {
	a.items[i], a.items[j] = a.items[j], a.items[i]
}
func (a *byExprString) Less(i, j int) bool {
	first, err := a.intr.Execute(a.node, a.items[i])
	if err != nil {
		a.hasError = true
		// Return a dummy value.
		return true
	}
	ith, ok := first.(string)
	if !ok {
		a.hasError = true
		return true
	}
	second, err := a.intr.Execute(a.node, a.items[j])
	if err != nil {
		a.hasError = true
		// Return a dummy value.
		return true
	}
	jth, ok := second.(string)
	if !ok {
		a.hasError = true
		return true
	}
	return ith < jth
}

type byExprFloat struct {
	intr     *treeInterpreter
	node     ASTNode
	items    []interface{}
	hasError bool
}

func (a *byExprFloat) Len() int {
	return len(a.items)
}
func (a *byExprFloat) Swap(i, j int) {
	a.items[i], a.items[j] = a.items[j], a.items[i]
}
func (a *byExprFloat) Less(i, j int) bool {
	first, err := a.intr.Execute(a.node, a.items[i])
	if err != nil {
		a.hasError = true
		// Return a dummy value.
		return true
	}
	ith, ok := first.(float64)
	if !ok {
		a.hasError = true
		return true
	}
	second, err := a.intr.Execute(a.node, a.items[j])
	if err != nil {
		a.hasError = true
		// Return a dummy value.
		return true
	}
	jth, ok := second.(float64)
	if !ok {
		a.hasError = true
		return true
	}
	return ith < jth
}

type functionCaller struct {
	functionTable map[string]FunctionEntry
}

func newFunctionCaller() *functionCaller {
	caller := &functionCaller{}
	caller.functionTable = map[string]FunctionEntry{
		"length": {
			name: "length",
			arguments: []ArgSpec{
				{types: []JPType{JPString, JPArray, JPObject}},
			},
			handler: JPfLength,
		},
		"starts_with": {
			name: "starts_with",
			arguments: []ArgSpec{
				{types: []JPType{JPString}},
				{types: []JPType{JPString}},
			},
			handler: JPfStartsWith,
		},
		"abs": {
			name: "abs",
			arguments: []ArgSpec{
				{types: []JPType{JPNumber}},
			},
			handler: JPfAbs,
		},
		"avg": {
			name: "avg",
			arguments: []ArgSpec{
				{types: []JPType{JPArrayNumber}},
			},
			handler: JPfAvg,
		},
		"ceil": {
			name: "ceil",
			arguments: []ArgSpec{
				{types: []JPType{JPNumber}},
			},
			handler: JPfCeil,
		},
		"contains": {
			name: "contains",
			arguments: []ArgSpec{
				{types: []JPType{JPArray, JPString}},
				{types: []JPType{JPAny}},
			},
			handler: JPfContains,
		},
		"ends_with": {
			name: "ends_with",
			arguments: []ArgSpec{
				{types: []JPType{JPString}},
				{types: []JPType{JPString}},
			},
			handler: JPfEndsWith,
		},
		"floor": {
			name: "floor",
			arguments: []ArgSpec{
				{types: []JPType{JPNumber}},
			},
			handler: JPfFloor,
		},
		"map": {
			name: "amp",
			arguments: []ArgSpec{
				{types: []JPType{JPExpref}},
				{types: []JPType{JPArray}},
			},
			handler:   JPfMap,
			hasExpRef: true,
		},
		"max": {
			name: "max",
			arguments: []ArgSpec{
				{types: []JPType{JPArrayNumber, JPArrayString}},
			},
			handler: JPfMax,
		},
		"merge": {
			name: "merge",
			arguments: []ArgSpec{
				{types: []JPType{JPObject}, variadic: true},
			},
			handler: JPfMerge,
		},
		"max_by": {
			name: "max_by",
			arguments: []ArgSpec{
				{types: []JPType{JPArray}},
				{types: []JPType{JPExpref}},
			},
			handler:   JPfMaxBy,
			hasExpRef: true,
		},
		"sum": {
			name: "sum",
			arguments: []ArgSpec{
				{types: []JPType{JPArrayNumber}},
			},
			handler: JPfSum,
		},
		"min": {
			name: "min",
			arguments: []ArgSpec{
				{types: []JPType{JPArrayNumber, JPArrayString}},
			},
			handler: JPfMin,
		},
		"min_by": {
			name: "min_by",
			arguments: []ArgSpec{
				{types: []JPType{JPArray}},
				{types: []JPType{JPExpref}},
			},
			handler:   JPfMinBy,
			hasExpRef: true,
		},
		"type": {
			name: "type",
			arguments: []ArgSpec{
				{types: []JPType{JPAny}},
			},
			handler: JPfType,
		},
		"keys": {
			name: "keys",
			arguments: []ArgSpec{
				{types: []JPType{JPObject}},
			},
			handler: JPfKeys,
		},
		"values": {
			name: "values",
			arguments: []ArgSpec{
				{types: []JPType{JPObject}},
			},
			handler: JPfValues,
		},
		"sort": {
			name: "sort",
			arguments: []ArgSpec{
				{types: []JPType{JPArrayString, JPArrayNumber}},
			},
			handler: JPfSort,
		},
		"sort_by": {
			name: "sort_by",
			arguments: []ArgSpec{
				{types: []JPType{JPArray}},
				{types: []JPType{JPExpref}},
			},
			handler:   JPfSortBy,
			hasExpRef: true,
		},
		"join": {
			name: "join",
			arguments: []ArgSpec{
				{types: []JPType{JPString}},
				{types: []JPType{JPArrayString}},
			},
			handler: JPfJoin,
		},
		"reverse": {
			name: "reverse",
			arguments: []ArgSpec{
				{types: []JPType{JPArray, JPString}},
			},
			handler: JPfReverse,
		},
		"to_array": {
			name: "to_array",
			arguments: []ArgSpec{
				{types: []JPType{JPAny}},
			},
			handler: JPfToArray,
		},
		"to_string": {
			name: "to_string",
			arguments: []ArgSpec{
				{types: []JPType{JPAny}},
			},
			handler: JPfToString,
		},
		"to_number": {
			name: "to_number",
			arguments: []ArgSpec{
				{types: []JPType{JPAny}},
			},
			handler: JPfToNumber,
		},
		"not_null": {
			name: "not_null",
			arguments: []ArgSpec{
				{types: []JPType{JPAny}, variadic: true},
			},
			handler: JPfNotNull,
		},
	}
	return caller
}

func (e *FunctionEntry) resolveArgs(arguments []interface{}) ([]interface{}, error) {
	if len(e.arguments) == 0 {
		return arguments, nil
	}
	if !e.arguments[len(e.arguments)-1].variadic {
		if len(e.arguments) != len(arguments) {
			return nil, errors.New("incorrect number of args")
		}
		for i, spec := range e.arguments {
			userArg := arguments[i]
			err := spec.typeCheck(userArg)
			if err != nil {
				return nil, err
			}
		}
		return arguments, nil
	}
	if len(arguments) < len(e.arguments) {
		return nil, errors.New("Invalid arity.")
	}
	return arguments, nil
}

func (a *ArgSpec) typeCheck(arg interface{}) error {
	for _, t := range a.types {
		switch t {
		case JPNumber:
			if _, ok := arg.(float64); ok {
				return nil
			}
		case JPString:
			if _, ok := arg.(string); ok {
				return nil
			}
		case JPArray:
			if isSliceType(arg) {
				return nil
			}
		case JPObject:
			if _, ok := arg.(map[string]interface{}); ok {
				return nil
			}
		case JPArrayNumber:
			if _, ok := toArrayNum(arg); ok {
				return nil
			}
		case JPArrayString:
			if _, ok := toArrayStr(arg); ok {
				return nil
			}
		case JPAny:
			return nil
		case JPExpref:
			if _, ok := arg.(expRef); ok {
				return nil
			}
		}
	}
	return fmt.Errorf("Invalid type for: %v, expected: %#v", arg, a.types)
}

func (f *functionCaller) AddCustomFunction(custom FunctionEntry) error {
	_, ok := f.functionTable[custom.name]
	if ok {
		return fmt.Errorf("function with name %s already defined", custom.name)
	}
	f.functionTable[custom.name] = custom
	return nil
}

func (f *functionCaller) CallFunction(name string, arguments []interface{}, intr *treeInterpreter) (interface{}, error) {
	entry, ok := f.functionTable[name]
	if !ok {
		return nil, errors.New("unknown function: " + name)
	}
	resolvedArgs, err := entry.resolveArgs(arguments)
	if err != nil {
		return nil, err
	}
	if entry.hasExpRef {
		var extra []interface{}
		extra = append(extra, intr)
		resolvedArgs = append(extra, resolvedArgs...)
	}
	return entry.handler(resolvedArgs)
}

func JPfAbs(arguments []interface{}) (interface{}, error) {
	num := arguments[0].(float64)
	return math.Abs(num), nil
}

func JPfLength(arguments []interface{}) (interface{}, error) {
	arg := arguments[0]
	if c, ok := arg.(string); ok {
		return float64(utf8.RuneCountInString(c)), nil
	} else if isSliceType(arg) {
		v := reflect.ValueOf(arg)
		return float64(v.Len()), nil
	} else if c, ok := arg.(map[string]interface{}); ok {
		return float64(len(c)), nil
	}
	return nil, errors.New("could not compute length()")
}

func JPfStartsWith(arguments []interface{}) (interface{}, error) {
	search := arguments[0].(string)
	prefix := arguments[1].(string)
	return strings.HasPrefix(search, prefix), nil
}

func JPfAvg(arguments []interface{}) (interface{}, error) {
	// We've already type checked the value so we can safely use
	// type assertions.
	args := arguments[0].([]interface{})
	length := float64(len(args))
	numerator := 0.0
	for _, n := range args {
		numerator += n.(float64)
	}
	return numerator / length, nil
}
func JPfCeil(arguments []interface{}) (interface{}, error) {
	val := arguments[0].(float64)
	return math.Ceil(val), nil
}
func JPfContains(arguments []interface{}) (interface{}, error) {
	search := arguments[0]
	el := arguments[1]
	if searchStr, ok := search.(string); ok {
		if elStr, ok := el.(string); ok {
			return strings.Contains(searchStr, elStr), nil
		}
		return false, nil
	}
	// Otherwise this is a generic contains for []interface{}
	general := search.([]interface{})
	for _, item := range general {
		if item == el {
			return true, nil
		}
	}
	return false, nil
}
func JPfEndsWith(arguments []interface{}) (interface{}, error) {
	search := arguments[0].(string)
	suffix := arguments[1].(string)
	return strings.HasSuffix(search, suffix), nil
}
func JPfFloor(arguments []interface{}) (interface{}, error) {
	val := arguments[0].(float64)
	return math.Floor(val), nil
}
func JPfMap(arguments []interface{}) (interface{}, error) {
	intr := arguments[0].(*treeInterpreter)
	exp := arguments[1].(expRef)
	node := exp.ref
	arr := arguments[2].([]interface{})
	mapped := make([]interface{}, 0, len(arr))
	for _, value := range arr {
		current, err := intr.Execute(node, value)
		if err != nil {
			return nil, err
		}
		mapped = append(mapped, current)
	}
	return mapped, nil
}
func JPfMax(arguments []interface{}) (interface{}, error) {
	if items, ok := toArrayNum(arguments[0]); ok {
		if len(items) == 0 {
			return nil, nil
		}
		if len(items) == 1 {
			return items[0], nil
		}
		best := items[0]
		for _, item := range items[1:] {
			if item > best {
				best = item
			}
		}
		return best, nil
	}
	// Otherwise we're dealing with a max() of strings.
	items, _ := toArrayStr(arguments[0])
	if len(items) == 0 {
		return nil, nil
	}
	if len(items) == 1 {
		return items[0], nil
	}
	best := items[0]
	for _, item := range items[1:] {
		if item > best {
			best = item
		}
	}
	return best, nil
}
func JPfMerge(arguments []interface{}) (interface{}, error) {
	final := make(map[string]interface{})
	for _, m := range arguments {
		mapped := m.(map[string]interface{})
		for key, value := range mapped {
			final[key] = value
		}
	}
	return final, nil
}
func JPfMaxBy(arguments []interface{}) (interface{}, error) {
	intr := arguments[0].(*treeInterpreter)
	arr := arguments[1].([]interface{})
	exp := arguments[2].(expRef)
	node := exp.ref
	if len(arr) == 0 {
		return nil, nil
	} else if len(arr) == 1 {
		return arr[0], nil
	}
	start, err := intr.Execute(node, arr[0])
	if err != nil {
		return nil, err
	}
	switch t := start.(type) {
	case float64:
		bestVal := t
		bestItem := arr[0]
		for _, item := range arr[1:] {
			result, err := intr.Execute(node, item)
			if err != nil {
				return nil, err
			}
			current, ok := result.(float64)
			if !ok {
				return nil, errors.New("invalid type, must be number")
			}
			if current > bestVal {
				bestVal = current
				bestItem = item
			}
		}
		return bestItem, nil
	case string:
		bestVal := t
		bestItem := arr[0]
		for _, item := range arr[1:] {
			result, err := intr.Execute(node, item)
			if err != nil {
				return nil, err
			}
			current, ok := result.(string)
			if !ok {
				return nil, errors.New("invalid type, must be string")
			}
			if current > bestVal {
				bestVal = current
				bestItem = item
			}
		}
		return bestItem, nil
	default:
		return nil, errors.New("invalid type, must be number of string")
	}
}
func JPfSum(arguments []interface{}) (interface{}, error) {
	items, _ := toArrayNum(arguments[0])
	sum := 0.0
	for _, item := range items {
		sum += item
	}
	return sum, nil
}

func JPfMin(arguments []interface{}) (interface{}, error) {
	if items, ok := toArrayNum(arguments[0]); ok {
		if len(items) == 0 {
			return nil, nil
		}
		if len(items) == 1 {
			return items[0], nil
		}
		best := items[0]
		for _, item := range items[1:] {
			if item < best {
				best = item
			}
		}
		return best, nil
	}
	items, _ := toArrayStr(arguments[0])
	if len(items) == 0 {
		return nil, nil
	}
	if len(items) == 1 {
		return items[0], nil
	}
	best := items[0]
	for _, item := range items[1:] {
		if item < best {
			best = item
		}
	}
	return best, nil
}

func JPfMinBy(arguments []interface{}) (interface{}, error) {
	intr := arguments[0].(*treeInterpreter)
	arr := arguments[1].([]interface{})
	exp := arguments[2].(expRef)
	node := exp.ref
	if len(arr) == 0 {
		return nil, nil
	} else if len(arr) == 1 {
		return arr[0], nil
	}
	start, err := intr.Execute(node, arr[0])
	if err != nil {
		return nil, err
	}
	if t, ok := start.(float64); ok {
		bestVal := t
		bestItem := arr[0]
		for _, item := range arr[1:] {
			result, err := intr.Execute(node, item)
			if err != nil {
				return nil, err
			}
			current, ok := result.(float64)
			if !ok {
				return nil, errors.New("invalid type, must be number")
			}
			if current < bestVal {
				bestVal = current
				bestItem = item
			}
		}
		return bestItem, nil
	} else if t, ok := start.(string); ok {
		bestVal := t
		bestItem := arr[0]
		for _, item := range arr[1:] {
			result, err := intr.Execute(node, item)
			if err != nil {
				return nil, err
			}
			current, ok := result.(string)
			if !ok {
				return nil, errors.New("invalid type, must be string")
			}
			if current < bestVal {
				bestVal = current
				bestItem = item
			}
		}
		return bestItem, nil
	} else {
		return nil, errors.New("invalid type, must be number of string")
	}
}
func JPfType(arguments []interface{}) (interface{}, error) {
	arg := arguments[0]
	if _, ok := arg.(float64); ok {
		return "number", nil
	}
	if _, ok := arg.(string); ok {
		return "string", nil
	}
	if _, ok := arg.([]interface{}); ok {
		return "array", nil
	}
	if _, ok := arg.(map[string]interface{}); ok {
		return "object", nil
	}
	if arg == nil {
		return "null", nil
	}
	if arg == true || arg == false {
		return "boolean", nil
	}
	return nil, errors.New("unknown type")
}
func JPfKeys(arguments []interface{}) (interface{}, error) {
	arg := arguments[0].(map[string]interface{})
	collected := make([]interface{}, 0, len(arg))
	for key := range arg {
		collected = append(collected, key)
	}
	return collected, nil
}
func JPfValues(arguments []interface{}) (interface{}, error) {
	arg := arguments[0].(map[string]interface{})
	collected := make([]interface{}, 0, len(arg))
	for _, value := range arg {
		collected = append(collected, value)
	}
	return collected, nil
}
func JPfSort(arguments []interface{}) (interface{}, error) {
	if items, ok := toArrayNum(arguments[0]); ok {
		d := sort.Float64Slice(items)
		sort.Stable(d)
		final := make([]interface{}, len(d))
		for i, val := range d {
			final[i] = val
		}
		return final, nil
	}
	// Otherwise we're dealing with sort()'ing strings.
	items, _ := toArrayStr(arguments[0])
	d := sort.StringSlice(items)
	sort.Stable(d)
	final := make([]interface{}, len(d))
	for i, val := range d {
		final[i] = val
	}
	return final, nil
}
func JPfSortBy(arguments []interface{}) (interface{}, error) {
	intr := arguments[0].(*treeInterpreter)
	arr := arguments[1].([]interface{})
	exp := arguments[2].(expRef)
	node := exp.ref
	if len(arr) == 0 {
		return arr, nil
	} else if len(arr) == 1 {
		return arr, nil
	}
	start, err := intr.Execute(node, arr[0])
	if err != nil {
		return nil, err
	}
	if _, ok := start.(float64); ok {
		sortable := &byExprFloat{intr, node, arr, false}
		sort.Stable(sortable)
		if sortable.hasError {
			return nil, errors.New("error in sort_by comparison")
		}
		return arr, nil
	} else if _, ok := start.(string); ok {
		sortable := &byExprString{intr, node, arr, false}
		sort.Stable(sortable)
		if sortable.hasError {
			return nil, errors.New("error in sort_by comparison")
		}
		return arr, nil
	} else {
		return nil, errors.New("invalid type, must be number of string")
	}
}
func JPfJoin(arguments []interface{}) (interface{}, error) {
	sep := arguments[0].(string)
	// We can't just do arguments[1].([]string), we have to
	// manually convert each item to a string.
	arrayStr := []string{}
	for _, item := range arguments[1].([]interface{}) {
		arrayStr = append(arrayStr, item.(string))
	}
	return strings.Join(arrayStr, sep), nil
}
func JPfReverse(arguments []interface{}) (interface{}, error) {
	if s, ok := arguments[0].(string); ok {
		r := []rune(s)
		for i, j := 0, len(r)-1; i < len(r)/2; i, j = i+1, j-1 {
			r[i], r[j] = r[j], r[i]
		}
		return string(r), nil
	}
	items := arguments[0].([]interface{})
	length := len(items)
	reversed := make([]interface{}, length)
	for i, item := range items {
		reversed[length-(i+1)] = item
	}
	return reversed, nil
}
func JPfToArray(arguments []interface{}) (interface{}, error) {
	if _, ok := arguments[0].([]interface{}); ok {
		return arguments[0], nil
	}
	return arguments[:1:1], nil
}
func JPfToString(arguments []interface{}) (interface{}, error) {
	if v, ok := arguments[0].(string); ok {
		return v, nil
	}
	result, err := json.Marshal(arguments[0])
	if err != nil {
		return nil, err
	}
	return string(result), nil
}
func JPfToNumber(arguments []interface{}) (interface{}, error) {
	arg := arguments[0]
	if v, ok := arg.(float64); ok {
		return v, nil
	}
	if v, ok := arg.(string); ok {
		conv, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, nil
		}
		return conv, nil
	}
	if _, ok := arg.([]interface{}); ok {
		return nil, nil
	}
	if _, ok := arg.(map[string]interface{}); ok {
		return nil, nil
	}
	if arg == nil {
		return nil, nil
	}
	if arg == true || arg == false {
		return nil, nil
	}
	return nil, errors.New("unknown type")
}
func JPfNotNull(arguments []interface{}) (interface{}, error) {
	for _, arg := range arguments {
		if arg != nil {
			return arg, nil
		}
	}
	return nil, nil
}
