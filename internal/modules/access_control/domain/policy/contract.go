package policy

import (
	"sort"
	"strings"

	"erg.ninja/pkg/auth"
)

type Portal string

const (
	PortalAll       Portal = "*"
	PortalLMS       Portal = "lms"
	PortalElearning Portal = "elearning"
	PortalHocLieu   Portal = "hoclieu"
	PortalCMS       Portal = "cms"
)

const (
	PermissionAll = "*"

	PermissionHocLieuAll             = "hoclieu.*"
	PermissionHocLieuContentRead     = "hoclieu.content.read"
	PermissionHocLieuContentCreate   = "hoclieu.content.create"
	PermissionHocLieuContentUpdate   = "hoclieu.content.update"
	PermissionHocLieuContentDelete   = "hoclieu.content.delete"
	PermissionHocLieuContentPublish  = "hoclieu.content.publish"
	PermissionHocLieuContentManage   = "hoclieu.content.manage"
	PermissionHocLieuProgramRead     = "hoclieu.program.read"
	PermissionHocLieuProgramManage   = "hoclieu.program.manage"
	PermissionHocLieuResourceRead    = "hoclieu.resource.read"
	PermissionHocLieuResourceCreate  = "hoclieu.resource.create"
	PermissionHocLieuResourceUpdate  = "hoclieu.resource.update"
	PermissionHocLieuResourceDelete  = "hoclieu.resource.delete"
	PermissionHocLieuResourcePub     = "hoclieu.resource.publish"
	PermissionHocLieuResourceArchive = "hoclieu.resource.archive"
	PermissionHocLieuAssetUpload     = "hoclieu.asset.upload"
	PermissionHocLieuAssetLaunch     = "hoclieu.asset.launch"
	PermissionHocLieuAssetDownload   = "hoclieu.asset.download"
	PermissionHocLieuAssetManage     = "hoclieu.asset.manage"
	PermissionHocLieuTaxonomyManage  = "hoclieu.taxonomy.manage"
	PermissionHocLieuViewerAuditRead = "hoclieu.viewer.audit.read"

	PermissionLMSAll                    = "lms.*"
	PermissionLMSScopeRead              = "lms.scope.read"
	PermissionLMSScopeUpdate            = "lms.scope.update"
	PermissionLMSUnitRead               = "lms.unit.read"
	PermissionLMSUnitCreate             = "lms.unit.create"
	PermissionLMSUnitUpdate             = "lms.unit.update"
	PermissionLMSCourseRead             = "lms.course.read"
	PermissionLMSCourseCreate           = "lms.course.create"
	PermissionLMSCourseUpdate           = "lms.course.update"
	PermissionLMSCourseDelete           = "lms.course.delete"
	PermissionLMSClassRead              = "lms.class.read"
	PermissionLMSClassManage            = "lms.class.manage"
	PermissionLMSClassCreate            = "lms.class.create"
	PermissionLMSClassUpdate            = "lms.class.update"
	PermissionLMSClassArchive           = "lms.class.archive"
	PermissionLMSStudentRead            = "lms.student.read"
	PermissionLMSStudentCreate          = "lms.student.create"
	PermissionLMSStudentUpdate          = "lms.student.update"
	PermissionLMSStudentImport          = "lms.student.import"
	PermissionLMSMemberRead             = "lms.member.read"
	PermissionLMSMemberInvite           = "lms.member.invite"
	PermissionLMSMemberUpdate           = "lms.member.update"
	PermissionLMSExamRead               = "lms.exam.read"
	PermissionLMSExamManage             = "lms.exam.manage"
	PermissionLMSQuestionRead           = "lms.question.read"
	PermissionLMSQuestionCreate         = "lms.question.create"
	PermissionLMSQuestionUpdate         = "lms.question.update"
	PermissionLMSQuestionDelete         = "lms.question.delete"
	PermissionLMSQuestionArchive        = "lms.question.archive"
	PermissionLMSQuizRead               = "lms.quiz.read"
	PermissionLMSQuizCreate             = "lms.quiz.create"
	PermissionLMSQuizUpdate             = "lms.quiz.update"
	PermissionLMSQuizPublish            = "lms.quiz.publish"
	PermissionLMSAssignmentRead         = "lms.assignment.read"
	PermissionLMSAssignmentCreate       = "lms.assignment.create"
	PermissionLMSAssignmentUpdate       = "lms.assignment.update"
	PermissionLMSAssignmentGrade        = "lms.assignment.grade"
	PermissionLMSGradeRead              = "lms.grade.read"
	PermissionLMSGradeUpdate            = "lms.grade.update"
	PermissionLMSAssignmentSubmit       = "lms.assignment.submit"
	PermissionLMSReportRead             = "lms.report.read"
	PermissionLMSPublicDisclosureManage = "lms.public_disclosure.manage"

	PermissionElearningAll                    = "elearning.*"
	PermissionElearningCourseRead             = "elearning.course.read"
	PermissionElearningCourseCreate           = "elearning.course.create"
	PermissionElearningCourseUpdate           = "elearning.course.update"
	PermissionElearningCourseDelete           = "elearning.course.delete"
	PermissionElearningCoursePublish          = "elearning.course.publish"
	PermissionElearningLessonRead             = "elearning.lesson.read"
	PermissionElearningLessonCreate           = "elearning.lesson.create"
	PermissionElearningLessonUpdate           = "elearning.lesson.update"
	PermissionElearningLessonDelete           = "elearning.lesson.delete"
	PermissionElearningEnrollmentRead         = "elearning.enrollment.read"
	PermissionElearningEnrollmentManage       = "elearning.enrollment.manage"
	PermissionElearningProfileReadSelf        = "elearning.profile.read_self"
	PermissionElearningAssignmentReadSelf     = "elearning.assignment.read_self"
	PermissionElearningAssignmentStartAttempt = "elearning.assignment.start_attempt"
	PermissionElearningAssignmentSaveAnswer   = "elearning.assignment.save_answer"
	PermissionElearningAssignmentSubmit       = "elearning.assignment.submit"
	PermissionElearningScoreReadSelf          = "elearning.score.read_self"
	PermissionElearningDiscussionRead         = "elearning.discussion.read"
	PermissionElearningDiscussionCreate       = "elearning.discussion.create"
	PermissionElearningDiscussionReply        = "elearning.discussion.reply"
	PermissionElearningNotificationReadSelf   = "elearning.notification.read_self"
	PermissionElearningNotificationUpdateSelf = "elearning.notification.update_self"

	PermissionRBACAll            = "rbac.*"
	PermissionRBACRoleRead       = "rbac.role.read"
	PermissionRBACRoleCreate     = "rbac.role.create"
	PermissionRBACRoleUpdate     = "rbac.role.update"
	PermissionRBACRoleDelete     = "rbac.role.delete"
	PermissionRBACRoleAssign     = "rbac.role.assign"
	PermissionRBACRoleManage     = "rbac.role.manage"
	PermissionRBACPermissionRead = "rbac.permission.read"
	PermissionRBACBindingManage  = "rbac.binding.manage"
	PermissionRBACOverrideManage = "rbac.override.manage"
	PermissionRBACPolicyRead     = "rbac.policy.read"
	PermissionRBACPolicyUpdate   = "rbac.policy.update"

	PermissionAuditRead = "audit.read"

	PermissionMediaAll    = "media.*"
	PermissionMediaRead   = "media.read"
	PermissionMediaUpload = "media.upload"
	PermissionMediaUpdate = "media.update"
	PermissionMediaDelete = "media.delete"
	PermissionMediaManage = "media.manage"
)

const (
	RoleSystemSuperAdmin = "system.super_admin"
	RoleERGSuperAdmin    = "erg_super_admin"
	RoleSystemAdmin      = "system.admin"
	RoleSystemRBACAdmin  = "system.rbac_admin"
	RoleSystemAuditor    = "system.audit_reader"
	RoleMediaManager     = "media.manager"

	RoleHocLieuAdmin  = "hoclieu.admin"
	RoleHocLieuEditor = "hoclieu.editor"
	RoleHocLieuViewer = "hoclieu.viewer"

	RoleLMSAdmin   = "lms.admin"
	RoleLMSTeacher = "lms.teacher"
	RoleLMSStudent = "lms.student"

	RoleElearningAdmin      = "elearning.admin"
	RoleElearningInstructor = "elearning.instructor"
	RoleElearningLearner    = "elearning.learner"
)

type PermissionDefinition struct {
	Name        string
	Group       string
	Label       string
	Description string
}

type RoleDefinition struct {
	Name        string
	Domain      string
	Description string
	Portals     []Portal
	Permissions []string
	IsDefault   bool
	Legacy      bool
}

func SupportedPortals() []Portal {
	return []Portal{PortalLMS, PortalElearning, PortalHocLieu, PortalCMS}
}

func EnterprisePermissionDefinitions() []PermissionDefinition {
	return []PermissionDefinition{
		{Name: PermissionHocLieuAll, Group: "hoclieu", Label: "Manage HocLieu"},
		{Name: PermissionHocLieuContentRead, Group: "hoclieu", Label: "View HocLieu content"},
		{Name: PermissionHocLieuContentCreate, Group: "hoclieu", Label: "Create HocLieu content"},
		{Name: PermissionHocLieuContentUpdate, Group: "hoclieu", Label: "Update HocLieu content"},
		{Name: PermissionHocLieuContentDelete, Group: "hoclieu", Label: "Delete HocLieu content"},
		{Name: PermissionHocLieuContentPublish, Group: "hoclieu", Label: "Publish HocLieu content"},
		{Name: PermissionHocLieuContentManage, Group: "hoclieu", Label: "Manage HocLieu workflows"},
		{Name: PermissionHocLieuProgramRead, Group: "hoclieu", Label: "View HocLieu programs"},
		{Name: PermissionHocLieuProgramManage, Group: "hoclieu", Label: "Manage HocLieu programs"},
		{Name: PermissionHocLieuResourceRead, Group: "hoclieu", Label: "View HocLieu resources"},
		{Name: PermissionHocLieuResourceCreate, Group: "hoclieu", Label: "Create HocLieu resources"},
		{Name: PermissionHocLieuResourceUpdate, Group: "hoclieu", Label: "Update HocLieu resources"},
		{Name: PermissionHocLieuResourceDelete, Group: "hoclieu", Label: "Delete HocLieu resources"},
		{Name: PermissionHocLieuResourcePub, Group: "hoclieu", Label: "Publish HocLieu resources"},
		{Name: PermissionHocLieuResourceArchive, Group: "hoclieu", Label: "Archive HocLieu resources"},
		{Name: PermissionHocLieuAssetUpload, Group: "hoclieu", Label: "Upload HocLieu assets"},
		{Name: PermissionHocLieuAssetLaunch, Group: "hoclieu", Label: "Launch HocLieu assets"},
		{Name: PermissionHocLieuAssetDownload, Group: "hoclieu", Label: "Download HocLieu assets"},
		{Name: PermissionHocLieuAssetManage, Group: "hoclieu", Label: "Manage HocLieu assets"},
		{Name: PermissionHocLieuTaxonomyManage, Group: "hoclieu", Label: "Manage HocLieu taxonomy"},
		{Name: PermissionHocLieuViewerAuditRead, Group: "hoclieu", Label: "View HocLieu audit"},

		{Name: PermissionLMSAll, Group: "lms", Label: "Manage LMS"},
		{Name: PermissionLMSScopeRead, Group: "lms", Label: "View LMS scope"},
		{Name: PermissionLMSScopeUpdate, Group: "lms", Label: "Update LMS scope"},
		{Name: PermissionLMSUnitRead, Group: "lms", Label: "View education units"},
		{Name: PermissionLMSUnitCreate, Group: "lms", Label: "Create education units"},
		{Name: PermissionLMSUnitUpdate, Group: "lms", Label: "Update education units"},
		{Name: PermissionLMSCourseRead, Group: "lms", Label: "View LMS courses"},
		{Name: PermissionLMSCourseCreate, Group: "lms", Label: "Create LMS courses"},
		{Name: PermissionLMSCourseUpdate, Group: "lms", Label: "Update LMS courses"},
		{Name: PermissionLMSCourseDelete, Group: "lms", Label: "Delete LMS courses"},
		{Name: PermissionLMSClassRead, Group: "lms", Label: "View LMS classes"},
		{Name: PermissionLMSClassManage, Group: "lms", Label: "Manage LMS classes"},
		{Name: PermissionLMSClassCreate, Group: "lms", Label: "Create LMS classes"},
		{Name: PermissionLMSClassUpdate, Group: "lms", Label: "Update LMS classes"},
		{Name: PermissionLMSClassArchive, Group: "lms", Label: "Archive LMS classes"},
		{Name: PermissionLMSStudentRead, Group: "lms", Label: "View students"},
		{Name: PermissionLMSStudentCreate, Group: "lms", Label: "Create students"},
		{Name: PermissionLMSStudentUpdate, Group: "lms", Label: "Update students"},
		{Name: PermissionLMSStudentImport, Group: "lms", Label: "Import student accounts"},
		{Name: PermissionLMSMemberRead, Group: "lms", Label: "View LMS members"},
		{Name: PermissionLMSMemberInvite, Group: "lms", Label: "Invite LMS members"},
		{Name: PermissionLMSMemberUpdate, Group: "lms", Label: "Update LMS members"},
		{Name: PermissionLMSExamRead, Group: "lms", Label: "View LMS exams"},
		{Name: PermissionLMSExamManage, Group: "lms", Label: "Manage LMS exams"},
		{Name: PermissionLMSQuestionRead, Group: "lms", Label: "View question bank"},
		{Name: PermissionLMSQuestionCreate, Group: "lms", Label: "Create questions"},
		{Name: PermissionLMSQuestionUpdate, Group: "lms", Label: "Update questions"},
		{Name: PermissionLMSQuestionDelete, Group: "lms", Label: "Delete questions"},
		{Name: PermissionLMSQuestionArchive, Group: "lms", Label: "Archive questions"},
		{Name: PermissionLMSQuizRead, Group: "lms", Label: "View quizzes"},
		{Name: PermissionLMSQuizCreate, Group: "lms", Label: "Create quizzes"},
		{Name: PermissionLMSQuizUpdate, Group: "lms", Label: "Update quizzes"},
		{Name: PermissionLMSQuizPublish, Group: "lms", Label: "Publish quizzes"},
		{Name: PermissionLMSAssignmentRead, Group: "lms", Label: "View assignments"},
		{Name: PermissionLMSAssignmentCreate, Group: "lms", Label: "Create assignments"},
		{Name: PermissionLMSAssignmentUpdate, Group: "lms", Label: "Update assignments"},
		{Name: PermissionLMSAssignmentGrade, Group: "lms", Label: "Grade assignments"},
		{Name: PermissionLMSGradeRead, Group: "lms", Label: "View LMS grades"},
		{Name: PermissionLMSGradeUpdate, Group: "lms", Label: "Update LMS grades"},
		{Name: PermissionLMSAssignmentSubmit, Group: "lms", Label: "Submit LMS assignments"},
		{Name: PermissionLMSReportRead, Group: "lms", Label: "View LMS reports"},
		{Name: PermissionLMSPublicDisclosureManage, Group: "lms", Label: "Manage public disclosure"},

		{Name: PermissionElearningAll, Group: "elearning", Label: "Manage Elearning"},
		{Name: PermissionElearningCourseRead, Group: "elearning", Label: "View Elearning courses"},
		{Name: PermissionElearningCourseCreate, Group: "elearning", Label: "Create Elearning courses"},
		{Name: PermissionElearningCourseUpdate, Group: "elearning", Label: "Update Elearning courses"},
		{Name: PermissionElearningCourseDelete, Group: "elearning", Label: "Delete Elearning courses"},
		{Name: PermissionElearningCoursePublish, Group: "elearning", Label: "Publish Elearning courses"},
		{Name: PermissionElearningLessonRead, Group: "elearning", Label: "View Elearning lessons"},
		{Name: PermissionElearningLessonCreate, Group: "elearning", Label: "Create Elearning lessons"},
		{Name: PermissionElearningLessonUpdate, Group: "elearning", Label: "Update Elearning lessons"},
		{Name: PermissionElearningLessonDelete, Group: "elearning", Label: "Delete Elearning lessons"},
		{Name: PermissionElearningEnrollmentRead, Group: "elearning", Label: "View Elearning enrollments"},
		{Name: PermissionElearningEnrollmentManage, Group: "elearning", Label: "Manage Elearning enrollments"},
		{Name: PermissionElearningProfileReadSelf, Group: "elearning", Label: "View own Elearning profile"},
		{Name: PermissionElearningAssignmentReadSelf, Group: "elearning", Label: "View own Elearning assignments"},
		{Name: PermissionElearningAssignmentStartAttempt, Group: "elearning", Label: "Start Elearning attempt"},
		{Name: PermissionElearningAssignmentSaveAnswer, Group: "elearning", Label: "Save Elearning answer"},
		{Name: PermissionElearningAssignmentSubmit, Group: "elearning", Label: "Submit Elearning assignment"},
		{Name: PermissionElearningScoreReadSelf, Group: "elearning", Label: "View own Elearning score"},
		{Name: PermissionElearningDiscussionRead, Group: "elearning", Label: "View Elearning discussions"},
		{Name: PermissionElearningDiscussionCreate, Group: "elearning", Label: "Create Elearning discussion"},
		{Name: PermissionElearningDiscussionReply, Group: "elearning", Label: "Reply to Elearning discussion"},
		{Name: PermissionElearningNotificationReadSelf, Group: "elearning", Label: "View own Elearning notifications"},
		{Name: PermissionElearningNotificationUpdateSelf, Group: "elearning", Label: "Update own Elearning notifications"},

		{Name: PermissionRBACAll, Group: "rbac", Label: "Manage RBAC"},
		{Name: PermissionRBACRoleRead, Group: "rbac", Label: "View roles"},
		{Name: PermissionRBACRoleCreate, Group: "rbac", Label: "Create roles"},
		{Name: PermissionRBACRoleUpdate, Group: "rbac", Label: "Update roles"},
		{Name: PermissionRBACRoleDelete, Group: "rbac", Label: "Delete roles"},
		{Name: PermissionRBACRoleAssign, Group: "rbac", Label: "Assign roles"},
		{Name: PermissionRBACRoleManage, Group: "rbac", Label: "Manage roles"},
		{Name: PermissionRBACPermissionRead, Group: "rbac", Label: "View permissions"},
		{Name: PermissionRBACBindingManage, Group: "rbac", Label: "Manage role bindings"},
		{Name: PermissionRBACOverrideManage, Group: "rbac", Label: "Manage permission overrides"},
		{Name: PermissionRBACPolicyRead, Group: "rbac", Label: "View access policies"},
		{Name: PermissionRBACPolicyUpdate, Group: "rbac", Label: "Update access policies"},

		{Name: PermissionAuditRead, Group: "audit", Label: "View audit logs"},

		{Name: PermissionMediaAll, Group: "media", Label: "Manage media"},
		{Name: PermissionMediaRead, Group: "media", Label: "View media"},
		{Name: PermissionMediaUpload, Group: "media", Label: "Upload media"},
		{Name: PermissionMediaUpdate, Group: "media", Label: "Update media"},
		{Name: PermissionMediaDelete, Group: "media", Label: "Delete media"},
		{Name: PermissionMediaManage, Group: "media", Label: "Manage media library"},
	}
}

func EnterpriseRoleDefinitions() []RoleDefinition {
	return []RoleDefinition{
		{
			Name:        RoleSystemSuperAdmin,
			Domain:      "system",
			Description: "Enterprise super administrator",
			Portals:     []Portal{PortalAll},
			Permissions: []string{PermissionAll},
		},
		{
			Name:        RoleSystemAdmin,
			Domain:      "system",
			Description: "Enterprise administrator",
			Portals:     []Portal{PortalAll},
			Permissions: []string{PermissionHocLieuAll, PermissionLMSAll, PermissionElearningAll, PermissionRBACAll, PermissionAuditRead, PermissionMediaAll, "cms.*"},
		},
		{
			Name:        RoleERGSuperAdmin,
			Domain:      "system",
			Description: "ERG platform super administrator",
			Portals:     []Portal{PortalAll},
			Permissions: []string{PermissionAll},
		},
		{
			Name:        RoleSystemRBACAdmin,
			Domain:      "system",
			Description: "RBAC administrator",
			Portals:     []Portal{PortalCMS},
			Permissions: []string{PermissionRBACAll, PermissionAuditRead},
		},
		{
			Name:        RoleSystemAuditor,
			Domain:      "system",
			Description: "Audit log reader",
			Portals:     []Portal{PortalCMS},
			Permissions: []string{PermissionAuditRead},
		},
		{
			Name:        RoleMediaManager,
			Domain:      "media",
			Description: "Media library administrator",
			Portals:     []Portal{PortalCMS, PortalLMS, PortalElearning, PortalHocLieu},
			Permissions: []string{PermissionMediaAll, PermissionHocLieuAssetUpload, PermissionHocLieuAssetDownload},
		},
		{
			Name:        RoleHocLieuAdmin,
			Domain:      "hoclieu",
			Description: "HocLieu administrator",
			Portals:     []Portal{PortalHocLieu},
			Permissions: []string{PermissionHocLieuAll, PermissionAuditRead},
		},
		{
			Name:        RoleHocLieuEditor,
			Domain:      "hoclieu",
			Description: "HocLieu editor",
			Portals:     []Portal{PortalHocLieu},
			Permissions: []string{PermissionHocLieuContentRead, PermissionHocLieuContentCreate, PermissionHocLieuContentUpdate, PermissionHocLieuContentPublish, PermissionHocLieuProgramRead, PermissionHocLieuResourceRead, PermissionHocLieuResourceCreate, PermissionHocLieuResourceUpdate, PermissionHocLieuResourcePub, PermissionHocLieuAssetUpload, PermissionHocLieuAssetLaunch, PermissionHocLieuAssetDownload},
		},
		{
			Name:        RoleHocLieuViewer,
			Domain:      "hoclieu",
			Description: "HocLieu viewer",
			Portals:     []Portal{PortalHocLieu},
			Permissions: []string{PermissionHocLieuContentRead, PermissionHocLieuProgramRead, PermissionHocLieuResourceRead, PermissionHocLieuAssetLaunch, PermissionHocLieuAssetDownload},
			IsDefault:   true,
		},
		{
			Name:        RoleLMSAdmin,
			Domain:      "lms",
			Description: "LMS administrator",
			Portals:     []Portal{PortalLMS},
			Permissions: []string{PermissionLMSAll, PermissionAuditRead},
		},
		{
			Name:        RoleLMSTeacher,
			Domain:      "lms",
			Description: "LMS teacher",
			Portals:     []Portal{PortalLMS},
			Permissions: []string{PermissionLMSScopeRead, PermissionLMSUnitRead, PermissionLMSCourseRead, PermissionLMSClassRead, PermissionLMSStudentRead, PermissionLMSExamRead, PermissionLMSExamManage, PermissionLMSQuestionRead, PermissionLMSQuestionCreate, PermissionLMSQuestionUpdate, PermissionLMSQuizRead, PermissionLMSQuizCreate, PermissionLMSQuizUpdate, PermissionLMSAssignmentRead, PermissionLMSAssignmentCreate, PermissionLMSAssignmentUpdate, PermissionLMSAssignmentGrade, PermissionLMSGradeRead, PermissionLMSGradeUpdate, PermissionLMSReportRead},
		},
		{
			Name:        RoleLMSStudent,
			Domain:      "lms",
			Description: "LMS student",
			Portals:     []Portal{PortalLMS},
			Permissions: []string{PermissionLMSScopeRead, PermissionLMSCourseRead, PermissionLMSClassRead, PermissionLMSAssignmentRead, PermissionLMSAssignmentSubmit, PermissionLMSGradeRead, PermissionLMSReportRead},
			IsDefault:   true,
		},
		{
			Name:        RoleElearningAdmin,
			Domain:      "elearning",
			Description: "Elearning administrator",
			Portals:     []Portal{PortalElearning},
			Permissions: []string{PermissionElearningAll, PermissionAuditRead},
		},
		{
			Name:        RoleElearningInstructor,
			Domain:      "elearning",
			Description: "Elearning instructor",
			Portals:     []Portal{PortalElearning},
			Permissions: []string{PermissionElearningCourseRead, PermissionElearningCourseCreate, PermissionElearningCourseUpdate, PermissionElearningCoursePublish, PermissionElearningLessonRead, PermissionElearningLessonCreate, PermissionElearningLessonUpdate},
		},
		{
			Name:        RoleElearningLearner,
			Domain:      "elearning",
			Description: "Elearning learner",
			Portals:     []Portal{PortalElearning},
			Permissions: []string{PermissionElearningProfileReadSelf, PermissionElearningAssignmentReadSelf, PermissionElearningAssignmentStartAttempt, PermissionElearningAssignmentSaveAnswer, PermissionElearningAssignmentSubmit, PermissionElearningScoreReadSelf, PermissionElearningDiscussionRead, PermissionElearningDiscussionCreate, PermissionElearningDiscussionReply, PermissionElearningNotificationReadSelf, PermissionElearningNotificationUpdateSelf},
			IsDefault:   true,
		},
	}
}

func LegacyRoleDefinitions() []RoleDefinition {
	return []RoleDefinition{
		{Name: "SUPER_ADMIN", Domain: "system", Description: "Legacy root administrator", Portals: []Portal{PortalAll}, Permissions: []string{PermissionAll}, Legacy: true},
		{Name: "admin", Domain: "system", Description: "Legacy administrator", Portals: []Portal{PortalAll}, Permissions: []string{PermissionAll}, Legacy: true},
		{Name: RoleERGSuperAdmin, Domain: "system", Description: "Legacy ERG root administrator", Portals: []Portal{PortalAll}, Permissions: []string{PermissionAll}, Legacy: true},
		{Name: "content_manager", Domain: "cms", Description: "Legacy CMS content manager", Portals: []Portal{PortalCMS}, Permissions: []string{"posts.*", "courses.*", "crawler.*", "seo.*"}, Legacy: true},
		{Name: "editor", Domain: "cms", Description: "Legacy CMS editor", Portals: []Portal{PortalCMS}, Permissions: []string{"posts.read", "posts.create", "posts.update", "posts.delete", "users.read"}, Legacy: true},
		{Name: "viewer", Domain: "cms", Description: "Legacy read-only viewer", Portals: []Portal{PortalCMS}, Permissions: []string{"*.read"}, Legacy: true},
		{Name: "teacher", Domain: "lms", Description: "Legacy LMS teacher", Portals: []Portal{PortalLMS, PortalHocLieu}, Permissions: []string{PermissionLMSScopeRead, PermissionLMSUnitRead, PermissionLMSCourseRead, PermissionLMSClassRead, PermissionLMSStudentRead, PermissionLMSExamRead, PermissionLMSQuestionRead, PermissionLMSQuizRead, PermissionLMSAssignmentRead, PermissionLMSAssignmentCreate, PermissionLMSGradeRead, PermissionLMSReportRead, PermissionHocLieuProgramRead, PermissionHocLieuResourceRead, PermissionHocLieuAssetLaunch, PermissionHocLieuAssetDownload}, Legacy: true},
		{Name: "student", Domain: "elearning", Description: "Legacy Elearning student", Portals: []Portal{PortalElearning}, Permissions: []string{PermissionElearningProfileReadSelf, PermissionElearningAssignmentReadSelf, PermissionElearningAssignmentStartAttempt, PermissionElearningAssignmentSaveAnswer, PermissionElearningAssignmentSubmit, PermissionElearningScoreReadSelf, PermissionElearningDiscussionRead, PermissionElearningDiscussionCreate, PermissionElearningDiscussionReply, PermissionElearningNotificationReadSelf, PermissionElearningNotificationUpdateSelf}, Legacy: true},
	}
}

func AllRoleDefinitions() []RoleDefinition {
	return append(EnterpriseRoleDefinitions(), LegacyRoleDefinitions()...)
}

func PermissionAliases(permission string) []string {
	switch strings.TrimSpace(permission) {
	case PermissionRBACRoleRead:
		return []string{"roles.read"}
	case PermissionRBACRoleCreate:
		return []string{"roles.create"}
	case PermissionRBACRoleUpdate:
		return []string{"roles.update"}
	case PermissionRBACRoleDelete:
		return []string{"roles.delete"}
	case PermissionRBACRoleAssign:
		return []string{"roles.assign"}
	case PermissionRBACPermissionRead:
		return []string{"roles.read"}
	case PermissionRBACOverrideManage:
		return []string{"roles.assign"}
	case PermissionAuditRead:
		return []string{"system.logs"}
	case "roles.read":
		return []string{PermissionRBACRoleRead, PermissionRBACPermissionRead}
	case "roles.create":
		return []string{PermissionRBACRoleCreate}
	case "roles.update":
		return []string{PermissionRBACRoleUpdate}
	case "roles.delete":
		return []string{PermissionRBACRoleDelete}
	case "roles.assign":
		return []string{PermissionRBACRoleAssign, PermissionRBACOverrideManage}
	case "system.logs":
		return []string{PermissionAuditRead}
	default:
		return nil
	}
}

func SubjectFromClaims(claims *auth.JWTClaims) Subject {
	if claims == nil {
		return Subject{}
	}
	subjectID := claims.UserID
	if subjectID == "" {
		subjectID = claims.Subject
	}
	if subjectID == "" {
		subjectID = claims.RegisteredClaims.Subject
	}
	subject := Subject{
		ID:                subjectID,
		TenantID:          claims.TenantID,
		Roles:             uniqueStrings(claims.Roles),
		Permissions:       uniqueStrings(claims.Permissions),
		DeniedPermissions: uniqueStrings(claims.DeniedPermissions),
		Attributes:        claims.Attributes,
	}
	portals := append([]string{}, claims.Portals...)
	if claims.Portal != "" {
		portals = append(portals, claims.Portal)
	}
	for _, portal := range PortalsForRoles(claims.Roles) {
		portals = append(portals, string(portal))
	}
	subject.Portals = normalizePortals(portals)
	return subject
}

func PermissionsForRoles(roles []string) []string {
	if len(roles) == 0 {
		return nil
	}
	byName := roleDefinitionByName()
	permissions := make([]string, 0)
	for _, role := range roles {
		def, ok := byName[normalizeRole(role)]
		if !ok {
			continue
		}
		permissions = append(permissions, def.Permissions...)
	}
	return uniqueStrings(permissions)
}

func PortalsForRoles(roles []string) []Portal {
	if len(roles) == 0 {
		return nil
	}
	byName := roleDefinitionByName()
	portals := make([]Portal, 0)
	for _, role := range roles {
		def, ok := byName[normalizeRole(role)]
		if !ok {
			continue
		}
		portals = append(portals, def.Portals...)
	}
	return uniquePortals(portals)
}

func RoleGrantsAllPermissions(roles []string) bool {
	return PermissionListMatches(PermissionsForRoles(roles), PermissionAll)
}

func ValidPortal(portal Portal) bool {
	portal = NormalizePortal(portal)
	if portal == PortalAll {
		return true
	}
	for _, supported := range SupportedPortals() {
		if portal == supported {
			return true
		}
	}
	return false
}

func NormalizePortal(portal Portal) Portal {
	return Portal(strings.ToLower(strings.TrimSpace(string(portal))))
}

func SubjectHasPortal(subject Subject, portal Portal) bool {
	required := NormalizePortal(portal)
	if required == "" {
		return true
	}
	portals := append([]Portal{}, subject.Portals...)
	portals = append(portals, PortalsForRoles(subject.Roles)...)
	for _, allowed := range portals {
		allowed = NormalizePortal(allowed)
		if allowed == PortalAll || allowed == required {
			return true
		}
	}
	return false
}

func roleDefinitionByName() map[string]RoleDefinition {
	defs := AllRoleDefinitions()
	byName := make(map[string]RoleDefinition, len(defs))
	for _, def := range defs {
		byName[normalizeRole(def.Name)] = def
	}
	return byName
}

func normalizeRole(role string) string {
	return strings.ToLower(strings.TrimSpace(role))
}

func normalizePortals(values []string) []Portal {
	portals := make([]Portal, 0, len(values))
	for _, value := range values {
		portal := NormalizePortal(Portal(value))
		if portal == "" {
			continue
		}
		portals = append(portals, portal)
	}
	return uniquePortals(portals)
}

func uniquePortals(values []Portal) []Portal {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[Portal]struct{}, len(values))
	result := make([]Portal, 0, len(values))
	for _, value := range values {
		value = NormalizePortal(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
