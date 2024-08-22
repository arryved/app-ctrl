package rbac

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"strings"

	"github.com/arryved/app-ctrl/api/config"
	"github.com/arryved/app-ctrl/api/secrets"
)

// used to make the authorizor function different for different targetUrn
type Authorizer func(
	ctx context.Context, cfg *config.Config, client interface{},
	principal config.PrincipalUrn, action config.Permission, target string) error

// AUTHORIZER generic, uses input to select a specific Authorizer
func Authorized(
	ctx context.Context, cfg *config.Config, client interface{},
	principal config.PrincipalUrn, action config.Permission, target string) error {
	// don't do authz if RBAC is turned off in cfg
	if !cfg.RBACEnabled {
		log.Warn("RBAC is disabled - everything is permitted!")
		return nil
	}

	// if the target is a secret urn, use the secret authorizer
	if strings.HasPrefix(target, "urn:arryved:secret") {
		return secrets.SecretsAuthorizer(ctx, cfg, client, principal, action, target)
	}

	// default authorizer is the config authorizer
	return ConfigAuthorizer(ctx, cfg, client, principal, action, target)
}

// AUTHORIZER for locally-configured access entries (e.g. deploy action)
func ConfigAuthorizer(ctx context.Context, cfg *config.Config, client interface{},
	principal config.PrincipalUrn, action config.Permission, target string) error {
	for _, entry := range cfg.AccessEntries {
		targetStr := string(entry.Target)
		if entry.Permission == action && (targetStr == target || targetStr == "*") {
			if PrincipalHasRole(cfg, principal, entry.Role) {
				return nil
			}
		}
	}
	return fmt.Errorf("not authorized principal=%s action=%s target=%s", principal, action, target)
}

// UTILITY for asserting role for principal
func PrincipalHasRole(cfg *config.Config, principal config.PrincipalUrn, role config.Role) bool {
	for _, group := range cfg.RoleMemberships[role] {
		if PrincipalInGroup(cfg, principal, group) {
			return true
		}
	}
	return false
}

// UTILITY for asserting group membership for principal
func PrincipalInGroup(cfg *config.Config, principal config.PrincipalUrn, group config.GroupUrn) bool {
	// TODO replace this with GWS group membership lookup
	for _, p := range cfg.UsersByGroups[group] {
		if p == principal {
			return true
		}
	}
	return false
}
