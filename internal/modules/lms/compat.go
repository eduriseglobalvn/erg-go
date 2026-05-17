package lms

import (
	lmscontroller "erg.ninja/internal/modules/lms/api/controller"
	lmsservice "erg.ninja/internal/modules/lms/application/service"
	lmsrepository "erg.ninja/internal/modules/lms/infrastructure/repository"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/storage"
)

type (
	Service    = lmsservice.Service
	Repository = lmsrepository.Repository
	Controller = lmscontroller.Controller
	Actor      = lmsservice.Actor

	CenterListRequestDTO = lmsservice.CenterListRequestDTO
	Center               = lmsservice.Center
)

func NewRepository(mongoClient *database.MongoClient) *Repository {
	return lmsrepository.NewRepository(mongoClient)
}

func NewService(repo *Repository, sheets *storage.GoogleSheetsClient, opts ...lmsservice.ServiceOption) *Service {
	return lmsservice.NewService(repo, sheets, opts...)
}

func NewController(svc *Service) *Controller {
	return lmscontroller.NewController(svc)
}

func newMemoryRepository() *Repository {
	return lmsrepository.NewMemoryRepository()
}
