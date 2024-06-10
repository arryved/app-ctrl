package rbac

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/arryved/app-ctrl/api/config"
)

func TestAuthorized(t *testing.T) {
	assert := assert.New(t)
	cfg := config.Load("../config/mock-config.yml")
	cfg.RBACEnabled = true

	assert.True(Authorized(cfg, "urn:example:user:alice.sre@example.com", "deploy", "urn:example:app:app1"))
	assert.True(Authorized(cfg, "urn:example:user:alice.sre@example.com", "deploy", "urn:example:app:app2"))
	assert.True(Authorized(cfg, "urn:example:user:alice.sre@example.com", "deploy", "urn:example:app:app3"))

	assert.True(Authorized(cfg, "urn:example:user:bob.dev@example.com", "deploy", "urn:example:app:app1"))
	assert.False(Authorized(cfg, "urn:example:user:bob.dev@example.com", "deploy", "urn:example:app:app2"))
	assert.False(Authorized(cfg, "urn:example:user:bob.dev@example.com", "deploy", "urn:example:app:app3"))

	assert.True(Authorized(cfg, "urn:example:user:angus.cto@example.com", "deploy", "urn:example:app:app1"))
	assert.True(Authorized(cfg, "urn:example:user:angus.cto@example.com", "deploy", "urn:example:app:app2"))
	assert.False(Authorized(cfg, "urn:example:user:angus.cto@example.com", "deploy", "urn:example:app:app3"))
}
