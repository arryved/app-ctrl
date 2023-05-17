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
	assert.Equal("info", config.LogLevel)
	assert.Equal(5, config.PollIntervalSec)
	assert.Len(config.AppDefs, 0)
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

	// check a deep healthz example
	assert.Len(config.AppDefs, 11)
	assert.Contains(config.AppDefs, "arryved-insider")
	assert.Len(config.AppDefs["arryved-insider"].Healthz, 2)
	assert.Equal(10999, config.AppDefs["arryved-insider"].Healthz[1].Port)
	assert.True(config.AppDefs["arryved-insider"].Healthz[1].TLS)

	// check a varz example
	assert.Equal(10998, config.AppDefs["arryved-insider"].Varz.Port)
	assert.False(config.AppDefs["arryved-insider"].Varz.TLS)
	assert.Equal("./api/mock_apt", config.AptPath)
}
