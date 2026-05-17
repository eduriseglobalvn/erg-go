package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const (
	accessScopeSystem = "system"
	accessScopeCenter = "center"
	accessScopeSchool = "school"
	systemScopeID     = "system"
)

var (
	errAccessManagementUnavailable = errors.New("ACCESS_MANAGEMENT_UNAVAILABLE")
	errInvalidAccessPolicy         = errors.New("INVALID_ACCESS_POLICY")
)

var accessModules = []AccessModuleDTO{
	{ID: "lms", Name: "LMS", Description: "Dashboard, lớp học, học sinh, bài tập, báo cáo"},
	{ID: "hoclieu", Name: "Học liệu", Description: "Thư viện học liệu, chương trình, tài nguyên số"},
	{ID: "media", Name: "Media", Description: "Quản lý nội dung media và tài nguyên hiển thị"},
}

var accessRoleGroups = []AccessRoleGroupDTO{
	{ID: "system_admin", Name: "System administrator", Description: "Quản trị toàn hệ thống ERG", ScopeTypes: []string{accessScopeSystem}, Permissions: []string{"*"}},
	{ID: "center_admin", Name: "Center administrator", Description: "Quản trị trung tâm, trường và lớp trực thuộc", ScopeTypes: []string{accessScopeCenter}, Permissions: []string{"lms.unit.read", "lms.unit.update", "lms.class.read", "lms.class.update", "lms.report.read", "lms.scope.read"}},
	{ID: "school_admin", Name: "School administrator", Description: "Quản trị lớp và học sinh trong trường", ScopeTypes: []string{accessScopeSchool}, Permissions: []string{"lms.class.read", "lms.class.update", "lms.report.read", "lms.assignment.read"}},
	{ID: "teacher", Name: "Teacher", Description: "Giảng dạy, giao bài, theo dõi lớp được phân công", ScopeTypes: []string{accessScopeSchool}, Permissions: []string{"lms.class.read", "lms.assignment.read", "lms.assignment.create", "lms.report.read"}},
	{ID: "media_manager", Name: "Media manager", Description: "Quản lý học liệu và media trong phạm vi được giao", ScopeTypes: []string{accessScopeSystem, accessScopeCenter}, Permissions: []string{"lms.question.read", "lms.question.create", "lms.quiz.read", "lms.quiz.create", "media.read", "media.create"}},
}

func (s *Service) AccessManagementOptions(ctx context.Context, tenantID string, actor Actor) (AccessManagementOptionsDTO, error) {
	managedCenterID, err := s.managedAccessCenterID(ctx, tenantID, actor)
	if err != nil {
		return AccessManagementOptionsDTO{}, err
	}
	if !actor.canAccessGlobal() && managedCenterID == "" {
		return AccessManagementOptionsDTO{}, errScopeForbidden
	}
	scopes, err := s.accessScopeOptions(ctx, tenantID, actor, managedCenterID)
	if err != nil {
		return AccessManagementOptionsDTO{}, err
	}
	return AccessManagementOptionsDTO{Scopes: scopes, RoleGroups: accessRoleGroupsForActor(actor), Modules: accessModules}, nil
}

func (s *Service) ListAccessScopes(ctx context.Context, tenantID string, actor Actor, scopeType, search string, page, limit int64) (AccessScopeListDTO, error) {
	managedCenterID, err := s.managedAccessCenterID(ctx, tenantID, actor)
	if err != nil {
		return AccessScopeListDTO{}, err
	}
	if !actor.canAccessGlobal() && managedCenterID == "" {
		return AccessScopeListDTO{}, errScopeForbidden
	}
	scopeType = strings.ToLower(strings.TrimSpace(scopeType))
	if scopeType == "" {
		scopeType = accessScopeCenter
	}
	if scopeType != accessScopeSystem && scopeType != accessScopeCenter && scopeType != accessScopeSchool {
		return AccessScopeListDTO{}, fmt.Errorf("%w: unsupported scope type", errInvalidAccessPolicy)
	}
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	if scopeType == accessScopeSystem {
		items := []AccessScopeOptionDTO{}
		if actor.canAccessGlobal() {
			items = append(items, systemAccessScopeOption())
		}
		return AccessScopeListDTO{Items: items, Total: int64(len(items)), Page: page, Limit: limit}, nil
	}
	reqType := educationUnitTypeCenter
	if scopeType == accessScopeSchool {
		reqType = educationUnitTypeSchool
	}
	centers, total, err := s.repo.ListCenters(ctx, tenantID, CenterListRequestDTO{
		Keyword: search,
		Type:    reqType,
		Page:    page,
		Limit:   limit,
	}, "")
	if err != nil {
		return AccessScopeListDTO{}, err
	}
	items := make([]AccessScopeOptionDTO, 0, len(centers))
	for _, center := range centers {
		if managedCenterID != "" {
			if scopeType == accessScopeCenter && center.ID.Hex() != managedCenterID {
				continue
			}
			if scopeType == accessScopeSchool && center.ParentID.Hex() != managedCenterID {
				continue
			}
		}
		items = append(items, accessScopeOptionForCenter(scopeType, center))
	}
	if managedCenterID != "" {
		total = int64(len(items))
	}
	return AccessScopeListDTO{Items: items, Total: total, Page: page, Limit: limit}, nil
}

func (s *Service) ListAccessManagedUsers(ctx context.Context, tenantID string, actor Actor, search, status, role string, page, limit int) (AccessManagementUserListDTO, error) {
	if ok, err := s.canManageAccess(ctx, tenantID, actor); err != nil {
		return AccessManagementUserListDTO{}, err
	} else if !ok {
		return AccessManagementUserListDTO{}, errScopeForbidden
	}
	if s.accessRepo == nil {
		return AccessManagementUserListDTO{}, errAccessManagementUnavailable
	}
	users, total, err := s.accessRepo.listUsers(ctx, tenantID, search, status, role, page, limit)
	if err != nil {
		return AccessManagementUserListDTO{}, err
	}
	ids := make([]string, 0, len(users))
	for _, user := range users {
		ids = append(ids, user.ID)
	}
	policyMap, err := s.accessPolicyMap(ctx, tenantID, ids)
	if err != nil {
		return AccessManagementUserListDTO{}, err
	}
	items := make([]AccessManagedUserDTO, 0, len(users))
	for _, user := range users {
		items = append(items, accessUserToDTO(user, policyMap[user.ID]))
	}
	return AccessManagementUserListDTO{Items: items, Total: total, Page: page, Limit: limit}, nil
}

func (s *Service) GetUserAccess(ctx context.Context, tenantID string, actor Actor, userID string) (UserAccessDetailDTO, error) {
	if ok, err := s.canManageAccess(ctx, tenantID, actor); err != nil {
		return UserAccessDetailDTO{}, err
	} else if !ok {
		return UserAccessDetailDTO{}, errScopeForbidden
	}
	if s.accessRepo == nil {
		return UserAccessDetailDTO{}, errAccessManagementUnavailable
	}
	user, err := s.accessRepo.getUser(ctx, tenantID, userID)
	if err != nil {
		return UserAccessDetailDTO{}, err
	}
	if user == nil {
		return UserAccessDetailDTO{}, errNotFound
	}
	policies, err := s.policiesForUser(ctx, tenantID, userID)
	if err != nil {
		return UserAccessDetailDTO{}, err
	}
	options, err := s.AccessManagementOptions(ctx, tenantID, actor)
	if err != nil {
		return UserAccessDetailDTO{}, err
	}
	return UserAccessDetailDTO{User: accessUserToDTO(*user, policies), Policies: policies, Effective: effectiveAccessForUser(*user, policies), Assignable: options}, nil
}

func (s *Service) SaveUserAccess(ctx context.Context, tenantID string, actor Actor, userID string, req SaveUserAccessRequestDTO) (UserAccessDetailDTO, error) {
	managedCenterID, err := s.managedAccessCenterID(ctx, tenantID, actor)
	if err != nil {
		return UserAccessDetailDTO{}, err
	}
	if !actor.canAccessGlobal() && managedCenterID == "" {
		return UserAccessDetailDTO{}, errScopeForbidden
	}
	if s.accessRepo == nil {
		return UserAccessDetailDTO{}, errAccessManagementUnavailable
	}
	if user, err := s.accessRepo.getUser(ctx, tenantID, userID); err != nil {
		return UserAccessDetailDTO{}, err
	} else if user == nil {
		return UserAccessDetailDTO{}, errNotFound
	}
	records := make([]userAccessRecord, 0, len(req.Policies))
	for _, policy := range req.Policies {
		normalized, err := s.normalizeAccessPolicy(ctx, tenantID, policy)
		if err != nil {
			return UserAccessDetailDTO{}, err
		}
		if !s.canAssignAccessPolicy(ctx, tenantID, actor, managedCenterID, normalized) {
			return UserAccessDetailDTO{}, fmt.Errorf("%w: policy outside actor management scope", errInvalidAccessPolicy)
		}
		records = append(records, userAccessRecord{ID: normalized.ID, UserID: userID, CenterID: centerIDForPolicy(normalized), Modules: normalized.Modules, Role: normalized.RoleGroup})
	}
	if err := s.accessRepo.replaceAccess(ctx, userID, records); err != nil {
		return UserAccessDetailDTO{}, err
	}
	return s.GetUserAccess(ctx, tenantID, actor, userID)
}

func (s *Service) PreviewUserAccess(ctx context.Context, tenantID string, actor Actor, req SaveUserAccessRequestDTO) (EffectiveAccessDTO, error) {
	managedCenterID, err := s.managedAccessCenterID(ctx, tenantID, actor)
	if err != nil {
		return EffectiveAccessDTO{}, err
	}
	if !actor.canAccessGlobal() && managedCenterID == "" {
		return EffectiveAccessDTO{}, errScopeForbidden
	}
	policies := make([]UserAccessPolicyDTO, 0, len(req.Policies))
	for _, policy := range req.Policies {
		normalized, err := s.normalizeAccessPolicy(ctx, tenantID, policy)
		if err != nil {
			return EffectiveAccessDTO{}, err
		}
		if !s.canAssignAccessPolicy(ctx, tenantID, actor, managedCenterID, normalized) {
			return EffectiveAccessDTO{}, fmt.Errorf("%w: policy outside actor management scope", errInvalidAccessPolicy)
		}
		policies = append(policies, normalized)
	}
	return effectiveAccess(policies), nil
}

func (s *Service) accessScopeOptions(ctx context.Context, tenantID string, actor Actor, managedCenterID string) ([]AccessScopeOptionDTO, error) {
	centers, _, err := s.repo.ListCenters(ctx, tenantID, CenterListRequestDTO{Page: 1, Limit: 100}, "")
	if err != nil {
		return nil, err
	}
	out := []AccessScopeOptionDTO{}
	if actor.canAccessGlobal() {
		out = append(out, AccessScopeOptionDTO{
			ScopeType: accessScopeSystem, ScopeID: systemScopeID, Name: "Hệ thống ERG", Badge: "System", Icon: "shield",
			Description: "Quản lý toàn hệ thống, trung tâm, trường học, lớp và phân quyền.",
		})
	}
	for _, center := range centers {
		if managedCenterID != "" && center.ID.Hex() != managedCenterID && center.ParentID.Hex() != managedCenterID {
			continue
		}
		scopeType := accessScopeCenter
		if centerType(center) == educationUnitTypeSchool {
			scopeType = accessScopeSchool
		}
		out = append(out, AccessScopeOptionDTO{
			ScopeType: scopeType, ScopeID: center.ID.Hex(), Name: center.Name,
			Badge: scopeBadge(centerType(center)), Icon: scopeIcon(centerType(center)),
			Description: hierarchyScopeDescription(scopeType, center.Name),
		})
	}
	return out, nil
}

func (s *Service) canManageAccess(ctx context.Context, tenantID string, actor Actor) (bool, error) {
	if actor.canAccessGlobal() {
		return true, nil
	}
	centerID, err := s.managedAccessCenterID(ctx, tenantID, actor)
	return centerID != "", err
}

func (s *Service) managedAccessCenterID(ctx context.Context, tenantID string, actor Actor) (string, error) {
	if actor.canAccessGlobal() {
		return "", nil
	}
	current, err := s.repo.GetCurrentScope(ctx, tenantID, actor.UserID)
	if err != nil {
		return "", err
	}
	if current != nil && normalizeScopeLevel(current.Level) == scopeLevelCenter && current.CenterID != "" && s.canAccessCenter(ctx, tenantID, actor, current.CenterID) {
		center, err := s.repo.GetCenter(ctx, tenantID, current.CenterID)
		if err != nil {
			return "", err
		}
		if center != nil && centerType(*center) == educationUnitTypeCenter {
			return center.ID.Hex(), nil
		}
	}
	centers, _, err := s.repo.ListCenters(ctx, tenantID, CenterListRequestDTO{Page: 1, Limit: 100}, actor.UserID)
	if err != nil {
		return "", err
	}
	for _, center := range centers {
		if centerType(center) == educationUnitTypeCenter {
			return center.ID.Hex(), nil
		}
	}
	return "", nil
}

func (s *Service) canAssignAccessPolicy(ctx context.Context, tenantID string, actor Actor, managedCenterID string, policy UserAccessPolicyDTO) bool {
	if actor.canAccessGlobal() {
		return true
	}
	if managedCenterID == "" || policy.ScopeType == accessScopeSystem {
		return false
	}
	center, err := s.repo.GetCenter(ctx, tenantID, policy.ScopeID)
	if err != nil || center == nil {
		return false
	}
	if policy.ScopeType == accessScopeCenter {
		return center.ID.Hex() == managedCenterID && centerType(*center) == educationUnitTypeCenter
	}
	return policy.ScopeType == accessScopeSchool && center.ParentID.Hex() == managedCenterID
}

func accessRoleGroupsForActor(actor Actor) []AccessRoleGroupDTO {
	if actor.canAccessGlobal() {
		return accessRoleGroups
	}
	out := make([]AccessRoleGroupDTO, 0, len(accessRoleGroups))
	for _, group := range accessRoleGroups {
		if group.ID == "system_admin" {
			continue
		}
		out = append(out, group)
	}
	return out
}

func (s *Service) accessPolicyMap(ctx context.Context, tenantID string, userIDs []string) (map[string][]UserAccessPolicyDTO, error) {
	out := map[string][]UserAccessPolicyDTO{}
	for _, userID := range userIDs {
		policies, err := s.policiesForUser(ctx, tenantID, userID)
		if err != nil {
			return nil, err
		}
		out[userID] = policies
	}
	return out, nil
}

func (s *Service) policiesForUser(ctx context.Context, tenantID string, userID string) ([]UserAccessPolicyDTO, error) {
	rows, err := s.accessRepo.listAccess(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]UserAccessPolicyDTO, 0, len(rows))
	for _, row := range rows {
		policy := UserAccessPolicyDTO{ID: row.ID, ScopeType: accessScopeSystem, ScopeID: systemScopeID, ScopeName: "Hệ thống ERG", RoleGroup: row.Role, Modules: row.Modules}
		if row.CenterID != systemScopeID {
			center, err := s.repo.GetCenter(ctx, tenantID, row.CenterID)
			if err != nil {
				return nil, err
			}
			if center == nil {
				continue
			}
			policy.ScopeID = center.ID.Hex()
			policy.ScopeName = center.Name
			policy.ScopeType = accessScopeCenter
			if centerType(*center) == educationUnitTypeSchool {
				policy.ScopeType = accessScopeSchool
			}
		}
		policy.Permissions = permissionsForRoleGroup(policy.RoleGroup)
		out = append(out, policy)
	}
	sortAccessPolicies(out)
	return out, nil
}

func (s *Service) normalizeAccessPolicy(ctx context.Context, tenantID string, policy UserAccessPolicyDTO) (UserAccessPolicyDTO, error) {
	policy.ScopeType = strings.ToLower(strings.TrimSpace(policy.ScopeType))
	policy.ScopeID = strings.TrimSpace(policy.ScopeID)
	policy.RoleGroup = strings.ToLower(strings.TrimSpace(policy.RoleGroup))
	policy.Modules = normalizeAccessModules(policy.Modules)
	if policy.ScopeType == accessScopeSystem {
		policy.ScopeID = systemScopeID
		policy.ScopeName = "Hệ thống ERG"
	} else {
		center, err := s.repo.GetCenter(ctx, tenantID, policy.ScopeID)
		if err != nil {
			return UserAccessPolicyDTO{}, err
		}
		if center == nil {
			return UserAccessPolicyDTO{}, fmt.Errorf("%w: scope not found", errInvalidAccessPolicy)
		}
		actualScope := accessScopeCenter
		if centerType(*center) == educationUnitTypeSchool {
			actualScope = accessScopeSchool
		}
		if policy.ScopeType != actualScope {
			return UserAccessPolicyDTO{}, fmt.Errorf("%w: scope type mismatch", errInvalidAccessPolicy)
		}
		policy.ScopeName = center.Name
	}
	if len(policy.Modules) == 0 {
		return UserAccessPolicyDTO{}, fmt.Errorf("%w: modules required", errInvalidAccessPolicy)
	}
	if !roleGroupAllowedForScope(policy.RoleGroup, policy.ScopeType) {
		return UserAccessPolicyDTO{}, fmt.Errorf("%w: role group is not allowed for scope", errInvalidAccessPolicy)
	}
	policy.Permissions = permissionsForRoleGroup(policy.RoleGroup)
	return policy, nil
}

func normalizeAccessModules(values []string) []string {
	allowed := map[string]struct{}{}
	for _, module := range accessModules {
		allowed[module.ID] = struct{}{}
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if _, ok := allowed[value]; ok {
			out = append(out, value)
		}
	}
	return uniqueStrings(out)
}

func roleGroupAllowedForScope(roleGroup, scopeType string) bool {
	for _, group := range accessRoleGroups {
		if group.ID != roleGroup {
			continue
		}
		for _, allowed := range group.ScopeTypes {
			if allowed == scopeType {
				return true
			}
		}
	}
	return false
}

func permissionsForRoleGroup(roleGroup string) []string {
	for _, group := range accessRoleGroups {
		if group.ID == roleGroup {
			return append([]string(nil), group.Permissions...)
		}
	}
	return nil
}

func centerIDForPolicy(policy UserAccessPolicyDTO) string {
	if policy.ScopeType == accessScopeSystem {
		return systemScopeID
	}
	return policy.ScopeID
}

func effectiveAccess(policies []UserAccessPolicyDTO) EffectiveAccessDTO {
	modules := []string{}
	permissions := []string{}
	highest := ""
	for _, policy := range policies {
		modules = append(modules, policy.Modules...)
		permissions = append(permissions, policy.Permissions...)
		if scopeRank(policy.ScopeType) > scopeRank(highest) {
			highest = policy.ScopeType
		}
	}
	if highest == "" {
		highest = "none"
	}
	return EffectiveAccessDTO{HighestScope: highest, Modules: uniqueStrings(modules), Permissions: uniqueStrings(permissions)}
}

func effectiveAccessForUser(user accessUserRecord, policies []UserAccessPolicyDTO) EffectiveAccessDTO {
	if hasSuperAdminAccessRole(user.Roles) {
		return EffectiveAccessDTO{HighestScope: accessScopeSystem, Modules: allAccessModuleIDs(), Permissions: []string{"*"}}
	}
	return effectiveAccess(policies)
}

func accessUserToDTO(user accessUserRecord, policies []UserAccessPolicyDTO) AccessManagedUserDTO {
	return AccessManagedUserDTO{
		ID: user.ID, Email: user.Email, FullName: user.FullName, AvatarURL: user.AvatarURL,
		Phone: user.Phone, Status: user.Status, AccountType: user.AccountType, Roles: user.Roles,
		IsProfileCompleted: user.IsProfileCompleted && strings.TrimSpace(user.FullName) != "" && strings.TrimSpace(user.Phone) != "", CreatedAt: user.CreatedAt.Format(timeFormatRFC3339()),
		AccessSummary: accessSummaryForUser(user, policies),
	}
}

func accessSummaryForUser(user accessUserRecord, policies []UserAccessPolicyDTO) AccessPolicySummaryDTO {
	if hasSuperAdminAccessRole(user.Roles) {
		return AccessPolicySummaryDTO{
			ScopeCount:   len(policies),
			Modules:      allAccessModuleIDs(),
			RoleGroups:   []string{"system_admin"},
			HighestScope: accessScopeSystem,
		}
	}
	return accessSummary(policies)
}

func accessSummary(policies []UserAccessPolicyDTO) AccessPolicySummaryDTO {
	modules := []string{}
	roleGroups := []string{}
	highest := ""
	for _, policy := range policies {
		modules = append(modules, policy.Modules...)
		roleGroups = append(roleGroups, policy.RoleGroup)
		if scopeRank(policy.ScopeType) > scopeRank(highest) {
			highest = policy.ScopeType
		}
	}
	if highest == "" {
		highest = "none"
	}
	return AccessPolicySummaryDTO{ScopeCount: len(policies), Modules: uniqueStrings(modules), RoleGroups: uniqueStrings(roleGroups), HighestScope: highest}
}

func hasSuperAdminAccessRole(roles []string) bool {
	for _, role := range roles {
		switch strings.ToLower(strings.TrimSpace(role)) {
		case "super_admin", "super-admin", "system.super_admin", "erg_super_admin":
			return true
		}
	}
	return false
}

func allAccessModuleIDs() []string {
	out := make([]string, 0, len(accessModules))
	for _, module := range accessModules {
		out = append(out, module.ID)
	}
	return out
}

func scopeRank(scopeType string) int {
	switch scopeType {
	case accessScopeSystem:
		return 3
	case accessScopeCenter:
		return 2
	case accessScopeSchool:
		return 1
	default:
		return 0
	}
}

func hierarchyScopeDescription(scopeType, name string) string {
	switch scopeType {
	case accessScopeCenter:
		return "Quản lý trường học và lớp trực thuộc " + name + "."
	case accessScopeSchool:
		return "Quản lý lớp học thuộc " + name + "."
	default:
		return "Quản lý toàn hệ thống ERG."
	}
}

func systemAccessScopeOption() AccessScopeOptionDTO {
	return AccessScopeOptionDTO{
		ScopeType:   accessScopeSystem,
		ScopeID:     systemScopeID,
		Name:        "Hệ thống ERG",
		Badge:       "System",
		Icon:        "shield",
		Description: "Quản lý toàn hệ thống, trung tâm, trường học, lớp và phân quyền.",
	}
}

func accessScopeOptionForCenter(scopeType string, center Center) AccessScopeOptionDTO {
	return AccessScopeOptionDTO{
		ScopeType:   scopeType,
		ScopeID:     center.ID.Hex(),
		Name:        center.Name,
		Badge:       scopeBadge(centerType(center)),
		Icon:        scopeIcon(centerType(center)),
		Description: hierarchyScopeDescription(scopeType, center.Name),
	}
}

func timeFormatRFC3339() string {
	return "2006-01-02T15:04:05Z07:00"
}

func sortAccessPolicies(policies []UserAccessPolicyDTO) {
	sort.Slice(policies, func(i, j int) bool {
		if scopeRank(policies[i].ScopeType) == scopeRank(policies[j].ScopeType) {
			return policies[i].ScopeName < policies[j].ScopeName
		}
		return scopeRank(policies[i].ScopeType) > scopeRank(policies[j].ScopeType)
	})
}
