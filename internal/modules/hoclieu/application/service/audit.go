package service

import (
	"context"

	auditservice "erg.ninja/internal/modules/audit/application/service"
	. "erg.ninja/internal/modules/hoclieu/api/dto"
)

// SetAuditPublisher wires the audit sink used by service-level hooks.
func (s *Service) SetAuditPublisher(publisher auditservice.Publisher) {
	if publisher == nil {
		publisher = auditservice.NoopPublisher{}
	}
	s.audit = publisher
}

func (s *Service) publishAssetAuditEvent(ctx context.Context, event auditservice.Event) {
	if s == nil || s.audit == nil {
		return
	}
	_ = s.audit.PublishAuditEvent(ctx, event)
}

func buildAssetAuditEvent(ctx context.Context, action string, asset AssetDTO, userID string, metadata map[string]any) auditservice.Event {
	return auditservice.BuildAssetEvent(auditservice.AssetEventInput{
		Action:      action,
		UserID:      userID,
		AssetID:     asset.ID,
		ResourceID:  asset.ResourceID,
		FileType:    string(asset.SelectedFileType),
		LaunchMode:  string(asset.LaunchMode),
		CanDownload: asset.CanDownload,
		Context:     auditservice.ContextFieldsFromContext(ctx),
		Metadata:    metadata,
	})
}

// BuildAssetDownloadAuditEvent exposes a controller-level hook for download audit.
func BuildAssetDownloadAuditEvent(ctx context.Context, asset AssetDTO, userID string) auditservice.Event {
	return auditservice.BuildAssetEvent(auditservice.AssetEventInput{
		Action:      auditservice.ActionAssetDownloaded,
		UserID:      userID,
		AssetID:     asset.ID,
		ResourceID:  asset.ResourceID,
		FileType:    string(asset.SelectedFileType),
		LaunchMode:  string(asset.LaunchMode),
		CanDownload: asset.CanDownload,
		Context:     auditservice.ContextFieldsFromContext(ctx),
	})
}
