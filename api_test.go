package jmespath

import (
	"encoding/json"
	"testing"

	"github.com/jmespath/go-jmespath/internal/testify/assert"
)

func TestValidUncompiledExpressionSearches(t *testing.T) {
	assert := assert.New(t)
	var j = []byte(`{"foo": {"bar": {"baz": [0, 1, 2, 3, 4]}}}`)
	var d interface{}
	err := json.Unmarshal(j, &d)
	assert.Nil(err)
	result, err := Search("foo.bar.baz[2]", d)
	assert.Nil(err)
	assert.Equal(2.0, result)
}

func TestValidPrecompiledExpressionSearches(t *testing.T) {
	assert := assert.New(t)
	data := make(map[string]interface{})
	data["foo"] = "bar"
	precompiled, err := Compile("foo")
	assert.Nil(err)
	result, err := precompiled.Search(data)
	assert.Nil(err)
	assert.Equal("bar", result)
}

func TestInvalidPrecompileErrors(t *testing.T) {
	assert := assert.New(t)
	_, err := Compile("not a valid expression")
	assert.NotNil(err)
}

func TestInvalidMustCompilePanics(t *testing.T) {
	defer func() {
		r := recover()
		assert.NotNil(t, r)
	}()
	MustCompile("not a valid expression")
}

func contains(s []string, i string) bool {
	for _, j := range s {
		if i == j {
			return true
		}
	}
	return false
}

func discard(arguments []interface{}) (interface{}, error) {
	arg1, _ := toArrayStr(arguments[0])
	arg2 := arguments[1]
	out := make([]string, 0)
	if c, ok := arg2.(string); ok {
		for _, i := range arg1 {
			if c != i {
				out = append(out, i)
			}
		}
	} else if isSliceType(arg2) {
		a, _ := toArrayStr(arg2)
		for _, i := range arg1 {
			if !contains(a, i) {
				out = append(out, i)
			}
		}
	}
	return out, nil
}

func TestCustomFunction(t *testing.T) {
	assert := assert.New(t)
	var j = []byte(`{"foo": ["bar", "test"]}`)
	d := make(map[string]interface{})
	err := json.Unmarshal(j, &d)
	assert.Nil(err)
	jp := NewJMESPath()
	err = jp.AddCustomFunction(FunctionEntry{
		name: "discard",
		arguments: []ArgSpec{
			{types: []JPType{JPArrayString}},
			{types: []JPType{JPArrayString, JPString}},
		},
		handler: discard,
	})
	assert.Nil(err)
	err = jp.SetExpression("foo | discard(@, 'test')")
	assert.Nil(err)
	result, err := jp.Search(d)
	assert.Nil(err)
	assert.Equal([]string{"bar"}, result)
}

func TestInvalidCustomFunction(t *testing.T) {
	assert := assert.New(t)
	jp := NewJMESPath()
	err := jp.AddCustomFunction(FunctionEntry{
		name: "length",
		arguments: []ArgSpec{
			{types: []JPType{JPArrayString}},
			{types: []JPType{JPArrayString, JPString}},
		},
		handler: discard,
	})
	assert.NotNil(err)
}
