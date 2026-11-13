package mongodb

type MongoDBRestaurantsRepo struct {
	*MongoDBRepo
}

func NewMongoDBRestaurantsRepo(repo *MongoDBRepo) *MongoDBRestaurantsRepo {
	return &MongoDBRestaurantsRepo{
		MongoDBRepo: repo,
	}
}
