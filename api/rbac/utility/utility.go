package utility

import (
	"github.com/arryved/app-ctrl/api/config"
)

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
