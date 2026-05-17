package policy

import "strings"

type Subject struct {
	ID                string
	TenantID          string
	Roles             []string
	Permissions       []string
	DeniedPermissions []string
	Portals           []Portal
	Attributes        map[string]string
}

type Resource struct {
	Namespace  string
	Type       string
	ID         string
	OwnerID    string
	TenantID   string
	Permission string
	Attributes map[string]string
}

type Scope struct {
	Portal     Portal
	TenantID   string
	Attributes map[string]string
}

type Request struct {
	Subject  Subject
	Action   string
	Resource Resource
	Scope    Scope
}

type Decision struct {
	Allowed            bool
	Reason             string
	RequiredPermission string
	MatchedPermission  string
}

func Decide(req Request) Decision {
	required := RequiredPermission(req.Action, req.Resource)
	if required == "" {
		return Decision{Reason: "permission required"}
	}

	if req.Scope.Portal != "" && !SubjectHasPortal(req.Subject, req.Scope.Portal) {
		return Decision{RequiredPermission: required, Reason: "portal denied"}
	}
	if req.Scope.TenantID != "" && req.Subject.TenantID != "" && req.Subject.TenantID != req.Scope.TenantID {
		return Decision{RequiredPermission: required, Reason: "tenant denied"}
	}
	if req.Resource.TenantID != "" && req.Subject.TenantID != "" && req.Subject.TenantID != req.Resource.TenantID {
		return Decision{RequiredPermission: required, Reason: "resource tenant denied"}
	}

	equivalents := permissionEquivalents(required)
	for _, candidate := range equivalents {
		if matched, ok := findMatchingPermission(req.Subject.DeniedPermissions, candidate); ok {
			return Decision{RequiredPermission: required, MatchedPermission: matched, Reason: "deny override"}
		}
	}

	grants := append([]string{}, req.Subject.Permissions...)
	grants = append(grants, PermissionsForRoles(req.Subject.Roles)...)
	for _, candidate := range equivalents {
		if matched, ok := findMatchingPermission(grants, candidate); ok {
			return Decision{Allowed: true, RequiredPermission: required, MatchedPermission: matched, Reason: "allowed"}
		}
	}

	return Decision{RequiredPermission: required, Reason: "permission denied"}
}

func RequiredPermission(action string, resource Resource) string {
	if resource.Permission != "" {
		return strings.TrimSpace(resource.Permission)
	}
	action = strings.Trim(strings.ToLower(action), ". ")
	if action == "" {
		return ""
	}
	if resource.Namespace == "" && resource.Type == "" {
		return action
	}

	parts := make([]string, 0, 3)
	if namespace := strings.Trim(strings.ToLower(resource.Namespace), ". "); namespace != "" {
		parts = append(parts, namespace)
	}
	if resourceType := strings.Trim(strings.ToLower(resource.Type), ". "); resourceType != "" {
		parts = append(parts, resourceType)
	}
	parts = append(parts, action)
	return strings.Join(parts, ".")
}

func PermissionListMatches(permissions []string, required string) bool {
	_, ok := findMatchingPermission(permissions, required)
	return ok
}

func PermissionMatches(granted, required string) bool {
	granted = strings.ToLower(strings.TrimSpace(granted))
	required = strings.ToLower(strings.TrimSpace(required))
	if granted == "" || required == "" {
		return false
	}
	if granted == PermissionAll {
		return true
	}
	if granted == required {
		return true
	}
	if strings.HasSuffix(granted, ".*") {
		prefix := strings.TrimSuffix(granted, "*")
		return strings.HasPrefix(required, prefix)
	}
	if granted == "*.read" && strings.HasSuffix(required, ".read") {
		return true
	}
	return false
}

func permissionEquivalents(required string) []string {
	values := append([]string{required}, PermissionAliases(required)...)
	return uniqueStrings(values)
}

func findMatchingPermission(permissions []string, required string) (string, bool) {
	for _, permission := range permissions {
		if PermissionMatches(permission, required) {
			return permission, true
		}
	}
	return "", false
}
