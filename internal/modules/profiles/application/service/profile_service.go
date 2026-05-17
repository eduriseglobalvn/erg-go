// Package service provides business logic for the profiles module.
package service

import (
	"context"
	"fmt"

	"erg.ninja/internal/modules/profiles/api/dto"
	entities "erg.ninja/internal/modules/profiles/domain/entity"
	"erg.ninja/internal/modules/profiles/infrastructure/repository"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

// Service provides profile business logic.
type Service struct {
	repo *repository.Repository
	log  *logger.Logger
}

// NewService creates a new profiles service.
func NewService(mongo *database.MongoClient, pg *database.GORMPostgresClient, log *logger.Logger) *Service {
	return &Service{
		repo: repository.NewRepository(mongo, pg, log),
		log:  log,
	}
}

// Repository returns the underlying repository.
func (s *Service) Repository() *repository.Repository {
	return s.repo
}

// BackfillLegacyProfiles migrates any remaining Mongo profile documents into
// PostgreSQL before the module starts serving traffic from the relational store.
func (s *Service) BackfillLegacyProfiles(ctx context.Context, mongo *database.MongoClient) error {
	return s.repo.BackfillFromMongo(ctx, mongo)
}

// ─── Profile Operations ─────────────────────────────────────────────────────

// CreateProfile creates a new user profile.
func (s *Service) CreateProfile(ctx context.Context, req dto.CreateProfileRequest) (*entities.Profile, error) {
	// Check if profile already exists.
	existing, err := s.repo.GetByUserID(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("profiles.CreateProfile: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("profiles.CreateProfile: profile already exists for this user")
	}

	profile := &entities.Profile{
		ID:          database.NewID(),
		UserID:      req.UserID,
		FullName:    req.FullName,
		Bio:         req.Bio,
		Phone:       req.Phone,
		DateOfBirth: req.DateOfBirth,
		Gender:      req.Gender,
		Address:     req.Address,
		City:        req.City,
		District:    req.District,
		SocialLinks: req.SocialLinks,
		AvatarURL:   req.AvatarURL,
	}

	if err := s.repo.Create(ctx, profile); err != nil {
		return nil, fmt.Errorf("profiles.CreateProfile: %w", err)
	}

	s.log.InfoContext(ctx).Str("user_id", profile.UserID).Msg("profiles: profile created")
	return profile, nil
}

// GetProfile returns a profile by user ID.
func (s *Service) GetProfile(ctx context.Context, userID string) (*entities.Profile, error) {
	profile, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("profiles.GetProfile: %w", err)
	}
	return profile, nil
}

// GetProfileByID returns a profile by document ID.
func (s *Service) GetProfileByID(ctx context.Context, id string) (*entities.Profile, error) {
	profile, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("profiles.GetProfileByID: %w", err)
	}
	return profile, nil
}

// UpdateProfile updates an existing profile.
func (s *Service) UpdateProfile(ctx context.Context, userID string, req dto.UpdateProfileRequest) (*entities.Profile, error) {
	updates := map[string]any{}
	if req.FullName != "" {
		updates["full_name"] = req.FullName
	}
	if req.Bio != "" {
		updates["bio"] = req.Bio
	}
	if req.Phone != "" {
		updates["phone"] = req.Phone
	}
	if req.DateOfBirth != nil {
		updates["date_of_birth"] = req.DateOfBirth
	}
	if req.Gender != "" {
		updates["gender"] = req.Gender
	}
	if req.Address != "" {
		updates["address"] = req.Address
	}
	if req.City != "" {
		updates["city"] = req.City
	}
	if req.District != "" {
		updates["district"] = req.District
	}
	if req.SocialLinks != "" {
		updates["social_links"] = req.SocialLinks
	}
	if req.AvatarURL != "" {
		updates["avatar_url"] = req.AvatarURL
	}

	if len(updates) == 0 {
		// No changes; return current profile.
		return s.repo.GetByUserID(ctx, userID)
	}

	profile, err := s.repo.Update(ctx, userID, updates)
	if err != nil {
		return nil, fmt.Errorf("profiles.UpdateProfile: %w", err)
	}
	if profile == nil {
		return nil, fmt.Errorf("profiles.UpdateProfile: profile not found")
	}

	s.log.InfoContext(ctx).Str("user_id", userID).Msg("profiles: profile updated")
	return profile, nil
}

// DeleteProfile deletes a profile by user ID.
func (s *Service) DeleteProfile(ctx context.Context, userID string) error {
	if err := s.repo.Delete(ctx, userID); err != nil {
		return fmt.Errorf("profiles.DeleteProfile: %w", err)
	}
	s.log.InfoContext(ctx).Str("user_id", userID).Msg("profiles: profile deleted")
	return nil
}

// GetMyProfile returns the profile for the authenticated user.
func (s *Service) GetMyProfile(ctx context.Context, userID string) (*entities.Profile, error) {
	return s.GetProfile(ctx, userID)
}

// UpsertProfile creates or updates a profile (upsert behavior).
func (s *Service) UpsertProfile(ctx context.Context, req dto.CreateProfileRequest) (*entities.Profile, error) {
	profile := &entities.Profile{
		ID:          database.NewID(),
		UserID:      req.UserID,
		FullName:    req.FullName,
		Bio:         req.Bio,
		Phone:       req.Phone,
		DateOfBirth: req.DateOfBirth,
		Gender:      req.Gender,
		Address:     req.Address,
		City:        req.City,
		District:    req.District,
		SocialLinks: req.SocialLinks,
		AvatarURL:   req.AvatarURL,
	}

	result, err := s.repo.Upsert(ctx, profile)
	if err != nil {
		return nil, fmt.Errorf("profiles.UpsertProfile: %w", err)
	}
	return result, nil
}
