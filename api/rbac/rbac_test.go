package rbac

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/arryved/app-ctrl/api/config"
)

func TestAuthorized(t *testing.T) {
	assert := assert.New(t)
	cfg := config.Load("../config/mock-config.yml")
	cfg.RBACEnabled = true
	ctx := context.Background()

	assert.NoError(Authorized(ctx, cfg, nil, "urn:example:user:alice.sre@example.com", "deploy", "urn:example:app:app1"))
	assert.NoError(Authorized(ctx, cfg, nil, "urn:example:user:alice.sre@example.com", "deploy", "urn:example:app:app2"))
	assert.NoError(Authorized(ctx, cfg, nil, "urn:example:user:alice.sre@example.com", "deploy", "urn:example:app:app3"))

	assert.NoError(Authorized(ctx, cfg, nil, "urn:example:user:bob.dev@example.com", "deploy", "urn:example:app:app1"))
	assert.Error(Authorized(ctx, cfg, nil, "urn:example:user:bob.dev@example.com", "deploy", "urn:example:app:app2"))
	assert.Error(Authorized(ctx, cfg, nil, "urn:example:user:bob.dev@example.com", "deploy", "urn:example:app:app3"))

	assert.NoError(Authorized(ctx, cfg, nil, "urn:example:user:angus.cto@example.com", "deploy", "urn:example:app:app1"))
	assert.NoError(Authorized(ctx, cfg, nil, "urn:example:user:angus.cto@example.com", "deploy", "urn:example:app:app2"))
	assert.Error(Authorized(ctx, cfg, nil, "urn:example:user:angus.cto@example.com", "deploy", "urn:example:app:app3"))
}
