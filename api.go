package jmespath

import (
	"fmt"
	"strconv"
)

// JMESPath is the representation of a compiled JMES path query. A JMESPath is
// safe for concurrent use by multiple goroutines.
type JMESPath struct {
	ast  *ASTNode
	intr *treeInterpreter
}

func NewJMESPath( ) *JMESPath {
	return &JMESPath{
		intr: newInterpreter(),
	}
}

func (jp *JMESPath) SetExpression(expression string) error {
	parser := NewParser()
	ast, err := parser.Parse(expression)
	if err != nil {
		return err
	}
	jp.ast = &ast
	return nil
}

func (jp *JMESPath) AddCustomFunction(custom FunctionEntry) error {
	return jp.intr.fCall.AddCustomFunction(custom)
}

// Search evaluates a JMESPath expression against input data and returns the result.
func (jp *JMESPath) SearchWithExpression(expression string, data interface{}) (interface{}, error) {
	parser := NewParser()
	ast, err := parser.Parse(expression)
	if err != nil {
		return nil, err
	}
	return jp.intr.Execute(ast, data)
}

// Search evaluates a JMESPath expression against input data and returns the result.
func (jp *JMESPath) Search(data interface{}) (interface{}, error) {
	if jp.ast == nil {
		return nil, fmt.Errorf("not expression set")
	}
	return jp.intr.Execute(*jp.ast, data)
}

// Compile parses a JMESPath expression and returns, if successful, a JMESPath
// object that can be used to match against data.
func Compile(expression string) (*JMESPath, error) {
	jmespath := NewJMESPath()
	err := jmespath.SetExpression(expression)
	if err != nil {
		return nil, err
	}
	return jmespath, nil
}

// MustCompile is like Compile but panics if the expression cannot be parsed.
// It simplifies safe initialization of global variables holding compiled
// JMESPaths.
func MustCompile(expression string) *JMESPath {
	jmespath, err := Compile(expression)
	if err != nil {
		panic(`jmespath: Compile(` + strconv.Quote(expression) + `): ` + err.Error())
	}
	return jmespath
}



// Search evaluates a JMESPath expression against input data and returns the result.
func Search(expression string, data interface{}) (interface{}, error) {
	jmespath := NewJMESPath()
	return jmespath.SearchWithExpression(expression ,data)
}
