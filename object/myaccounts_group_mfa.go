package object

import (
	"errors"
	"strings"
)

const GroupMfaProperty = "myaccounts.requireMfa"

// Selected groups require an authenticator factor, including enabled ancestors.
// This policy uses existing persisted group properties and upstream MFA setup.
func IsGroupMfaRequired(user *User) (bool, error) {
	if user == nil {
		return false, nil
	}
	pending := append([]string(nil), user.Groups...)
	seen := map[string]bool{}
	for len(pending) > 0 {
		if len(seen) > 1000 {
			return false, errors.New("group security hierarchy is too deep")
		}
		id := pending[0]
		pending = pending[1:]
		if seen[id] {
			continue
		}
		seen[id] = true
		if !strings.HasPrefix(id, user.Owner+"/") {
			return false, errors.New("group security policy belongs to another organization")
		}
		group, e := GetGroup(id)
		if e != nil {
			return false, e
		}
		if group == nil || !group.IsEnabled {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(group.Properties[GroupMfaProperty]), "true") {
			return true, nil
		}
		if group.ParentId != "" {
			parent := group.ParentId
			if !strings.Contains(parent, "/") {
				parent = group.Owner + "/" + parent
			}
			pending = append(pending, parent)
		}
	}
	return false, nil
}

// Re-evaluate inherited policy and revoke issuer sessions when enforcement changes.
// Existing client sessions follow the standard back-channel/SSO logout support.
func enforceGroupMfaTransition(oldGroup, group *Group) error {
	if oldGroup.Properties[GroupMfaProperty] == group.Properties[GroupMfaProperty] && oldGroup.IsEnabled == group.IsEnabled && oldGroup.ParentId == group.ParentId {
		return nil
	}
	users, err := GetUsers(group.Owner)
	if err != nil {
		return err
	}
	for _, user := range users {
		required, err := IsGroupMfaRequired(user)
		if err != nil {
			return err
		}
		if required {
			if err = terminateUserAccess(user); err != nil {
				return err
			}
		}
	}
	return nil
}
