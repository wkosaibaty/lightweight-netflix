package repositories

import (
	"context"
	"errors"

	"github.com/wkosaibaty/lightweight-netflix/models"
	"github.com/wkosaibaty/lightweight-netflix/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	lookupStage  primitive.D
	projectStage primitive.D
)

func init() {
	lookupStage = bson.D{
		{
			Key: "$lookup",
			Value: bson.D{
				{
					Key:   "from",
					Value: "watchedMovies",
				},
				{
					Key:   "localField",
					Value: "_id",
				},
				{
					Key:   "foreignField",
					Value: "movieId",
				},
				{
					Key:   "as",
					Value: "watchedMovies",
				},
				{
					Key: "pipeline",
					Value: []bson.M{
						{
							"$match": bson.M{
								"rate": bson.M{
									"$gt": 0,
								},
							},
						},
					},
				},
			},
		},
	}

	projectStage = bson.D{{Key: "$project", Value: bson.M{
		"name":        1,
		"description": 1,
		"date":        1,
		"cover":       1,
		"userId":      1,
		"rate":        bson.M{"$avg": "$watchedMovies.rate"},
	}}}
}

type MovieRepositoryImpl struct {
	movieCollection *mongo.Collection
	ctx             context.Context
}

func NewMovieRepository(movieCollection *mongo.Collection, ctx context.Context) MovieRepository {
	return &MovieRepositoryImpl{movieCollection, ctx}
}

func (repository *MovieRepositoryImpl) FindAllMovies(sortBy string, sortType int) ([]*models.Movie, error) {
	pipeline := []bson.D{lookupStage, projectStage}

	if sortBy != "" {
		sortStage := bson.D{{Key: "$sort", Value: bson.M{sortBy: sortType}}}
		pipeline = []bson.D{lookupStage, projectStage, sortStage}
	}

	cursor, err := repository.movieCollection.Aggregate(repository.ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(repository.ctx)

	var movies []*models.Movie

	for cursor.Next(repository.ctx) {
		movie := &models.Movie{}
		err := cursor.Decode(movie)
		if err != nil {
			return nil, err
		}
		movies = append(movies, movie)
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	if len(movies) == 0 {
		return []*models.Movie{}, nil
	}
	return movies, nil
}

func (repository *MovieRepositoryImpl) FindMovieById(id string) (*models.Movie, error) {
	objectId, _ := primitive.ObjectIDFromHex(id)
	var movie *models.Movie

	matchStage := bson.D{{Key: "$match", Value: bson.D{{Key: "_id", Value: objectId}}}}
	cursor, err := repository.movieCollection.Aggregate(repository.ctx, mongo.Pipeline{matchStage, lookupStage, projectStage})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(repository.ctx)

	if !cursor.Next(repository.ctx) {
		return nil, errors.New("Movie with the given id does not exist")
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	if err = cursor.Decode(&movie); err != nil {
		return nil, err
	}

	return movie, nil
}

func (repository *MovieRepositoryImpl) CreateMovie(request *models.CreateMovieRequest) (*models.Movie, error) {
	res, err := repository.movieCollection.InsertOne(repository.ctx, request)
	if err != nil {
		return nil, err
	}

	var movie *models.Movie
	if err = repository.movieCollection.FindOne(repository.ctx, bson.M{"_id": res.InsertedID}).Decode(&movie); err != nil {
		return nil, err
	}

	return movie, nil
}

func (repository *MovieRepositoryImpl) UpdateMovie(id string, request *models.UpdateMovieRequest) (*models.Movie, error) {
	document, err := utils.ToDocument(request)
	if err != nil {
		return nil, err
	}

	objectId, _ := primitive.ObjectIDFromHex(id)
	query := bson.D{{Key: "_id", Value: objectId}}
	update := bson.D{{Key: "$set", Value: document}}
	result := repository.movieCollection.FindOneAndUpdate(repository.ctx, query, update, options.FindOneAndUpdate().SetReturnDocument(1))

	var movie *models.Movie
	if err := result.Decode(&movie); err != nil {
		return nil, errors.New("Movie with the given id does not exist")
	}

	return movie, nil
}

func (repository *MovieRepositoryImpl) DeleteMovie(id string) error {
	objectId, _ := primitive.ObjectIDFromHex(id)

	result, err := repository.movieCollection.DeleteOne(repository.ctx, bson.M{"_id": objectId})
	if err != nil {
		return err
	}

	if result.DeletedCount == 0 {
		return errors.New("Movie with the given id does not exist")
	}

	return nil
}
