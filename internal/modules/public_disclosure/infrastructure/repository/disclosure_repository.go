package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"erg.ninja/internal/modules/public_disclosure/domain/entity"
	"erg.ninja/pkg/database"
)

type Repository struct {
	db *database.MongoClient
}

func NewRepository(db *database.MongoClient) *Repository {
	return &Repository{db: db}
}

func (r *Repository) collection() *mongo.Collection {
	return r.db.Database().Collection(entities.DisclosureCollection)
}

func (r *Repository) List(ctx context.Context, tenantID string, sectionSlug string) ([]entities.DisclosureDocument, error) {
	filter := bson.M{"tenant_id": tenantID}
	if sectionSlug != "" {
		filter["section_slug"] = sectionSlug
	}

	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := r.collection().Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var items []entities.DisclosureDocument
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) Create(ctx context.Context, doc *entities.DisclosureDocument) error {
	if doc.ID == "" {
		doc.ID = database.NewID()
	}
	doc.CreatedAt = time.Now()
	doc.UpdatedAt = doc.CreatedAt

	_, err := r.collection().InsertOne(ctx, doc)
	return err
}

func (r *Repository) FindByID(ctx context.Context, tenantID, id string) (*entities.DisclosureDocument, error) {
	filter := bson.M{"_id": id, "tenant_id": tenantID}
	var doc entities.DisclosureDocument
	if err := r.collection().FindOne(ctx, filter).Decode(&doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

func (r *Repository) Update(ctx context.Context, doc *entities.DisclosureDocument) error {
	doc.UpdatedAt = time.Now()
	filter := bson.M{"_id": doc.ID, "tenant_id": doc.TenantID}
	update := bson.M{"$set": doc}
	_, err := r.collection().UpdateOne(ctx, filter, update)
	return err
}

func (r *Repository) Delete(ctx context.Context, tenantID, id string) error {
	filter := bson.M{"_id": id, "tenant_id": tenantID}
	_, err := r.collection().DeleteOne(ctx, filter)
	return err
}
