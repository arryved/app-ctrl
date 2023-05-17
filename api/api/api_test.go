//go:build !integration

package api

import (
	"crypto/tls"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCipherSuitesFromConfig(t *testing.T) {
	assert := assert.New(t)

	configuredCiphers := []string{
		"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA",
		"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
		"TLS_RSA_WITH_AES_256_CBC_SHA",
	}
	expectedCiphers := []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_RSA_WITH_AES_256_CBC_SHA,
	}

	sort.Slice(expectedCiphers, func(i, j int) bool {
		return expectedCiphers[i] < expectedCiphers[j]
	})

	actualCiphers := CipherSuitesFromConfig(configuredCiphers)

	sort.Slice(actualCiphers, func(i, j int) bool {
		return actualCiphers[i] < actualCiphers[j]
	})

	assert.Equal(actualCiphers, expectedCiphers)
}

func TestTLSVersionFromConfig(t *testing.T) {
	assert := assert.New(t)

	actualVersion := TLSVersionFromConfig("1.2")

	assert.Equal(uint16(tls.VersionTLS12), actualVersion)
}
