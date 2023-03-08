//go:build !integration

package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// check that status returns correct struct with a mocked set of downstream responses
// for apt, ps, etc.
func TestSerialize(t *testing.T) {
	assert := assert.New(t)
	status := Status{
		Versions: Versions{
			Config:    -1,
			Installed: &Version{Major: 1, Minor: 2, Patch: 3},
			Running:   &Version{Major: 4, Minor: 5, Patch: 6},
		},
	}

	actual, err := json.Marshal(status)

	assert.Nil(err)
	assert.Equal(
		"{\"versions\":{\"config\":-1,\"installed\":{\"major\":1,\"minor\":2,"+
			"\"patch\":3,\"build\":0},\"running\":{\"major\":4,\"minor\":5,\"patch\":6,\"build\":0}},"+
			"\"health\":null}",
		string(actual))
}

// check version parsing examples
func TestParseVersion(t *testing.T) {
	assert := assert.New(t)

	examples := []string{
		"2.14.2",
		"1.8-345",
		"0.7-0",
		"1.0.0-20220123",
		"1.0",
	}

	expected := []Version{
		Version{Major: 2, Minor: 14, Patch: 2, Build: -1},
		Version{Major: 1, Minor: 8, Patch: -1, Build: 345},
		Version{Major: 0, Minor: 7, Patch: -1, Build: 0},
		Version{Major: 1, Minor: 0, Patch: 0, Build: 20220123},
		Version{Major: 1, Minor: 0, Patch: -1, Build: -1},
	}

	// these cases should all work
	for i := range examples {
		actual, err := ParseVersion(examples[i])
		assert.Nil(err)
		assert.Equal(expected[i], actual)
	}

	// this case should not work (extra hyphen)
	_, err := ParseVersion("1.1.1-100-200")
	assert.NotNil(err)

	// this case should not work (extra dot)
	_, err = ParseVersion("1.1.1.1-100")
	assert.NotNil(err)

	// this case should not work (non-number)
	_, err = ParseVersion("1.1.1.a-100")
	assert.NotNil(err)
}
