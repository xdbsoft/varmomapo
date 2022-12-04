package mongodb

import (
	"context"
	"log"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type Client struct {
	client   *mongo.Client
	database *mongo.Database
}

func New(ctx context.Context, uri string, database string) (*Client, error) {

	log.Print("connecting to datastore")
	// Create a new client and connect to the server
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(uri))

	if err != nil {
		return nil, err
	}

	// Ping the primary
	if err := client.Ping(context.TODO(), readpref.Primary()); err != nil {
		return nil, err
	}

	log.Println("Successfully connected and pinged.")

	d := client.Database(database)

	return &Client{
		client:   client,
		database: d,
	}, nil
}

func (c *Client) Close(ctx context.Context) error {
	return c.client.Disconnect(ctx)
}

func (c *Client) Insert(ctx context.Context, collection string, features []*geojson.Feature) error {
	data := make([]any, len(features))
	for i, f := range features {
		data[i] = f
	}
	col := c.database.Collection(collection)
	res, err := col.InsertMany(ctx, data)
	if err != nil {
		return err
	}
	//log.Print("Insert succeed: ", len(res.InsertedIDs))
	_ = res
	return nil
}

func (c *Client) Get(ctx context.Context, collection string, filter bson.D, doc any) error {
	res := c.database.Collection(collection).FindOne(ctx, filter)
	return res.Decode(doc)
}

func (c *Client) Put(ctx context.Context, collection string, doc any) error {
	_, err := c.database.Collection(collection).InsertOne(ctx, doc)
	return err
}

func (c *Client) Inc(ctx context.Context, collection string, filter bson.D, key string) error {

	_, err := c.database.Collection(collection).UpdateOne(ctx, filter, bson.D{
		{"$inc", bson.D{{key, 1}}},
	})
	return err
}

func And(filters ...*bson.D) *bson.D {
	expressions := make(bson.A, len(filters))
	for i := range filters {
		expressions[i] = *filters[i]
	}
	return &bson.D{
		{"$and", expressions},
	}
}

func (c *Client) FindInBBox(ctx context.Context, collection string, bound orb.Bound, filter *bson.D) ([]*geojson.Feature, error) {
	coll := c.database.Collection(collection)
	match := bson.D{
		{"geometry", bson.D{
			{"$geoWithin", bson.D{
				{"$geometry", bson.D{
					{"type", "Polygon"},
					{"coordinates", bson.A{bson.A{
						bson.A{bound.Left(), bound.Bottom()},
						bson.A{bound.Left(), bound.Top()},
						bson.A{bound.Right(), bound.Top()},
						bson.A{bound.Right(), bound.Bottom()},
						bson.A{bound.Left(), bound.Bottom()},
					}}},
				}},
			}},
		}},
	}

	if filter != nil {
		match = *And(&match, filter)
	}

	cursor, err := coll.Aggregate(ctx, bson.A{
		bson.D{
			{"$match", match},
		},
	})
	if err != nil {
		return nil, err
	}

	var results []*geojson.Feature
	for cursor.Next(ctx) {
		result := geojson.Feature{}

		result.ID = cursor.Current.Lookup("id").Int64()
		result.Type = cursor.Current.Lookup("type").StringValue()
		result.Geometry = orb.Point{
			cursor.Current.Lookup("geometry").Array().Index(0).Value().Double(),
			cursor.Current.Lookup("geometry").Array().Index(1).Value().Double(),
		}
		result.Properties = geojson.Properties(convertToMap(cursor.Current.Lookup("properties").Document()))

		results = append(results, &result)
	}
	return results, nil
}

func convertToMap(d bson.Raw) map[string]any {
	m := make(map[string]any)
	elements, err := d.Elements()
	if err != nil {
		log.Fatal(err)
	}
	for _, e := range elements {
		k := e.Key()

		switch e.Value().Type {
		case bsontype.Int32:
			m[k] = e.Value().Int32()
		case bsontype.Int64:
			m[k] = e.Value().Int64()
		case bsontype.DateTime:
			m[k] = e.Value().DateTime()
		case bsontype.String:
			m[k] = e.Value().StringValue()
		case bsontype.EmbeddedDocument:
			embedded := e.Value().Document()
			m[k] = convertToMap(embedded)
		default:
			log.Print("Unsupported data type for", k, e.Value())
		}
	}
	return m
}
