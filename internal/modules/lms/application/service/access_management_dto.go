package service

type AccessManagedUserDTO struct {
	ID                 string                 `json:"id"`
	Email              string                 `json:"email"`
	FullName           string                 `json:"fullName"`
	AvatarURL          string                 `json:"avatarUrl,omitempty"`
	Phone              string                 `json:"phone,omitempty"`
	Status             string                 `json:"status"`
	AccountType        string                 `json:"accountType,omitempty"`
	Roles              []string               `json:"roles"`
	IsProfileCompleted bool                   `json:"isProfileCompleted"`
	AccessSummary      AccessPolicySummaryDTO `json:"accessSummary"`
	CreatedAt          string                 `json:"createdAt"`
}

type AccessPolicySummaryDTO struct {
	ScopeCount   int      `json:"scopeCount"`
	Modules      []string `json:"modules"`
	RoleGroups   []string `json:"roleGroups"`
	HighestScope string   `json:"highestScope"`
}

type AccessManagementUserListDTO struct {
	Items []AccessManagedUserDTO `json:"items"`
	Total int64                  `json:"total"`
	Page  int                    `json:"page"`
	Limit int                    `json:"limit"`
}

type AccessScopeOptionDTO struct {
	ScopeType   string `json:"scopeType"`
	ScopeID     string `json:"scopeId"`
	Name        string `json:"name"`
	Badge       string `json:"badge"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
}

type AccessRoleGroupDTO struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	ScopeTypes  []string `json:"scopeTypes"`
	Permissions []string `json:"permissions"`
}

type AccessModuleDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type AccessManagementOptionsDTO struct {
	Scopes     []AccessScopeOptionDTO `json:"scopes"`
	RoleGroups []AccessRoleGroupDTO   `json:"roleGroups"`
	Modules    []AccessModuleDTO      `json:"modules"`
}

type AccessScopeListDTO struct {
	Items []AccessScopeOptionDTO `json:"items"`
	Total int64                  `json:"total"`
	Page  int64                  `json:"page"`
	Limit int64                  `json:"limit"`
}

type UserAccessPolicyDTO struct {
	ID          string   `json:"id,omitempty"`
	ScopeType   string   `json:"scopeType"`
	ScopeID     string   `json:"scopeId"`
	ScopeName   string   `json:"scopeName,omitempty"`
	RoleGroup   string   `json:"roleGroup"`
	Modules     []string `json:"modules"`
	Permissions []string `json:"permissions,omitempty"`
}

type UserAccessDetailDTO struct {
	User       AccessManagedUserDTO       `json:"user"`
	Policies   []UserAccessPolicyDTO      `json:"policies"`
	Effective  EffectiveAccessDTO         `json:"effective"`
	Assignable AccessManagementOptionsDTO `json:"assignable"`
}

type EffectiveAccessDTO struct {
	HighestScope string   `json:"highestScope"`
	Modules      []string `json:"modules"`
	Permissions  []string `json:"permissions"`
	Warnings     []string `json:"warnings,omitempty"`
}

type SaveUserAccessRequestDTO struct {
	Policies []UserAccessPolicyDTO `json:"policies"`
}
