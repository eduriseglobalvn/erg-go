package service

import (
	"context"
	"fmt"
	"mime/multipart"

	"erg.ninja/internal/modules/documents"
	docDto "erg.ninja/internal/modules/documents/dto"
	"erg.ninja/internal/modules/public_disclosure/entities"
	"erg.ninja/internal/modules/public_disclosure/repository"
	"erg.ninja/pkg/logger"
)

type Service struct {
	repo   *repository.Repository
	docSvc *documents.Service
	log    *logger.Logger
}

func NewService(repo *repository.Repository, docSvc *documents.Service, log *logger.Logger) *Service {
	return &Service{
		repo:   repo,
		docSvc: docSvc,
		log:    log,
	}
}

func (s *Service) List(ctx context.Context, tenantID, section string) ([]entities.DisclosureDocument, error) {
	return s.repo.List(ctx, tenantID, section)
}

func (s *Service) Create(ctx context.Context, doc *entities.DisclosureDocument, file *multipart.FileHeader, wmDTO docDto.WatermarkConfigDTO) (*entities.DisclosureDocument, error) {
	// 1. Upload file using documents service
	// We use GDrive for public disclosure
	uploadedDoc, err := s.docSvc.Upload(ctx, doc.TenantID, file, doc.ID, wmDTO, true)
	if err != nil {
		return nil, fmt.Errorf("disclosure.service: upload failed: %w", err)
	}

	doc.DocumentID = uploadedDoc.ID
	doc.WatermarkConfig = wmDTO.ToEntity()

	// 2. Save disclosure metadata
	if err := s.repo.Create(ctx, doc); err != nil {
		return nil, fmt.Errorf("disclosure.service: create metadata failed: %w", err)
	}

	return doc, nil
}

func (s *Service) GetByID(ctx context.Context, tenantID, id string) (*entities.DisclosureDocument, error) {
	return s.repo.FindByID(ctx, tenantID, id)
}

func (s *Service) Delete(ctx context.Context, tenantID, id string) error {
	// Optional: Delete associated document as well
	return s.repo.Delete(ctx, tenantID, id)
}
