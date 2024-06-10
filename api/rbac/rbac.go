package rbac

import (
	log "github.com/sirupsen/logrus"
	"github.com/arryved/app-ctrl/api/config"
)

func Authorized(cfg *config.Config, principal config.PrincipalUrn, action config.Permission, target string) bool {
	if !cfg.RBACEnabled {
		log.Warn("RBAC is disabled - everything is permitted!")
		return true
	}
	for _, entry := range cfg.AccessEntries {
		targetStr := string(entry.Target)
		if entry.Permission == action && (targetStr == target || targetStr == "*") {
			if PrincipalHasRole(cfg, principal, entry.Role) {
				return true
			}
		}
	}
	return false
}

func PrincipalHasRole(cfg *config.Config, principal config.PrincipalUrn, role config.Role) bool {
	for _, group := range cfg.RoleMemberships[role] {
		if PrincipalInGroup(cfg, principal, group) {
			return true
		}
	}
	return false
}

func PrincipalInGroup(cfg *config.Config, principal config.PrincipalUrn, group config.GroupUrn) bool {
	// TODO replace this with GWS group membership lookup
	for _, p := range cfg.UsersByGroups[group] {
		if p == principal {
			return true
		}
	}
	return false
}
