//go:build !integration

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecutorFake(t *testing.T) {
	assert := assert.New(t)
	executor := MockExecutor{
		MockOutput: "hello",
		MockError:  nil,
	}

	output, err := executor.Run(nil)

	assert.Nil(err)
	assert.NotEmpty(output)
}

func TestExecutorReal(t *testing.T) {
	assert := assert.New(t)
	executor := Executor{
		Command: "/bin/test",
		Args:    []string{"0"},
	}

	output, err := executor.Run(nil)

	assert.Nil(err)
	assert.Empty(output)
}
