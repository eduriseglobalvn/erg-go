package repository

import (
	lmsservice "erg.ninja/internal/modules/lms/application/service"
	"erg.ninja/pkg/database"
)

type Repository = lmsservice.Repository

func NewRepository(mongoClient *database.MongoClient) *Repository {
	return lmsservice.NewRepository(mongoClient)
}

func NewMemoryRepository() *Repository {
	return lmsservice.NewMemoryRepository()
}
