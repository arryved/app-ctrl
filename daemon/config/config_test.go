//go:build !integration

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadDefaults(t *testing.T) {
	assert := assert.New(t)

	config := Load("/etc/app-controld.yml")

	assert.Equal(1024, config.Port)
	assert.Equal("./var/service.key", config.KeyPath)
	assert.Equal("./var/service.crt", config.CrtPath)
	assert.Equal(10, config.ReadTimeoutS)
	assert.Equal(10, config.WriteTimeoutS)
	assert.Equal("/usr/bin/apt", config.AptPath)
}

func TestLoadFile(t *testing.T) {
	assert := assert.New(t)

	// this should actually exist as a fixture on disk
	// and set ReadTimeoutS, AptPath to something other
	// than the default
	config := Load("./mock-config.yml")

	assert.Equal(1024, config.Port)
	assert.Equal("./var/service.key", config.KeyPath)
	assert.Equal("./var/service.crt", config.CrtPath)
	assert.Equal(30, config.ReadTimeoutS)
	assert.Equal(10, config.WriteTimeoutS)
	assert.Equal("apt", config.AptPath)
}
