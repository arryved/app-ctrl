package gke

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNoop(t *testing.T) {
	assert := assert.New(t)
	assert.True(true)
	// TODO add mocked cases for apply
}
