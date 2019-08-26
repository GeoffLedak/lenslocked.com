package models

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	ErrUserIDRequired modelError = "models: user ID is required"
	ErrTitleRequired  modelError = "models: title is required"
)

// Gallery represents the galleries table in our DB
// and is mostly a container resource composed of images.
type Gallery struct {
	ID        primitive.ObjectID `bson:"_id"`
	UserID    primitive.ObjectID
	Title     string
	Images    []Image
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}

func (g *Gallery) ImagesSplitN(n int) [][]Image {
	ret := make([][]Image, n)
	for i := 0; i < n; i++ {
		ret[i] = make([]Image, 0)
	}
	for i, img := range g.Images {
		bucket := i % n
		ret[bucket] = append(ret[bucket], img)
	}
	return ret
}

func NewGalleryService(db *mongo.Client) GalleryService {
	return &galleryService{
		GalleryDB: &galleryValidator{
			GalleryDB: &galleryGorm{
				db: db,
			},
		},
	}
}

type GalleryService interface {
	GalleryDB
}

type galleryService struct {
	GalleryDB
}

// GalleryDB is used to interact with the galleries database.
//
// For pretty much all single gallery queries:
// If the gallery is found, we will return a nil error
// If the gallery is not found, we will return ErrNotFound
// If there is another error, we will return an error with
// more information about what went wrong. This may not be
// an error generated by the models package.
type GalleryDB interface {
	ByID(id string) (*Gallery, error)
	ByUserID(userID string) ([]Gallery, error)
	Create(gallery *Gallery) error
	Update(gallery *Gallery) error
	Delete(id string) error
}

type galleryValidator struct {
	GalleryDB
}

func (gv *galleryValidator) Create(gallery *Gallery) error {
	err := runGalleryValFns(gallery,
		gv.userIDRequired,
		gv.titleRequired)
	if err != nil {
		return err
	}
	return gv.GalleryDB.Create(gallery)
}

func (gv *galleryValidator) Update(gallery *Gallery) error {
	err := runGalleryValFns(gallery,
		gv.userIDRequired,
		gv.titleRequired)
	if err != nil {
		return err
	}
	return gv.GalleryDB.Update(gallery)
}

func (gv *galleryValidator) Delete(id string) error {

	/*
		var gallery Gallery
		gallery.ID = ObjectIdHex(id)
		if err := runGalleryValFns(&gallery, gv.nonZeroID); err != nil {
			return err
		}
	*/

	return gv.GalleryDB.Delete(id)
}

// I dont think this needs to be here
// it's just one of those silly unused vars
// for making sure stuff can be initialized properly
// var _ GalleryDB = &galleryGorm{}

type galleryGorm struct {
	db *mongo.Client
}

func (gg *galleryGorm) ByID(id string) (*Gallery, error) {
	collection := gg.db.Database("lenslocked_dev").Collection("galleries")
	var gallery Gallery
	filter := bson.D{{"_id", id}}
	err := collection.FindOne(context.TODO(), filter).Decode(&gallery)
	if err != nil {
		return nil, err
	}
	return &gallery, nil
}

func (gg *galleryGorm) ByUserID(userID string) ([]Gallery, error) {
	collection := gg.db.Database("lenslocked_dev").Collection("galleries")
	var galleries []Gallery

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.D{{"user_id", userID}}

	cursor, err := collection.Find(ctx, filter)

	if err != nil {
		return nil, err
	}

	for cursor.Next(ctx) {
		var gallery Gallery
		err := cursor.Decode(&gallery)
		if err != nil {
			return nil, err
		}

		galleries = append(galleries, gallery)
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	return galleries, nil
}

func (gg *galleryGorm) Create(gallery *Gallery) error {
	collection := gg.db.Database("lenslocked_dev").Collection("galleries")
	_, err := collection.InsertOne(context.TODO(), gallery)
	return err
}

func (gg *galleryGorm) Update(gallery *Gallery) error {
	collection := gg.db.Database("lenslocked_dev").Collection("galleries")
	filter := bson.D{{"_id", gallery.ID}}
	_, err := collection.UpdateOne(context.TODO(), filter, gallery)
	return err
}

func (gg *galleryGorm) Delete(id string) error {
	collection := gg.db.Database("lenslocked_dev").Collection("galleries")
	filter := bson.D{{"_id", id}}
	_, err := collection.DeleteOne(context.TODO(), filter)
	return err
}

func (gv *galleryValidator) userIDRequired(g *Gallery) error {

	/*
		if g.UserID <= 0 {
			return ErrUserIDRequired
		}
	*/

	return nil
}

func (gv *galleryValidator) titleRequired(g *Gallery) error {
	if g.Title == "" {
		return ErrTitleRequired
	}
	return nil
}

func (gv *galleryValidator) nonZeroID(gallery *Gallery) error {

	/*
		if gallery.ID <= 0 {
			return ErrIDInvalid
		}
	*/

	return nil
}

type galleryValFn func(*Gallery) error

func runGalleryValFns(gallery *Gallery, fns ...galleryValFn) error {
	for _, fn := range fns {
		if err := fn(gallery); err != nil {
			return err
		}
	}
	return nil
}
