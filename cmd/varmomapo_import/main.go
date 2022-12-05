package main

import (
	"context"
	"log"
	"os"
	"sort"

	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/osm"
	"github.com/paulmach/osm/osmpbf"
	"github.com/schollz/progressbar/v3"

	"github.com/xdbsoft/varmomapo/mongodb"
)

var mainTags map[string]bool = map[string]bool{
	"amenity":              true,
	"bench":                true,
	"disused:amenity":      true,
	"planned:amenity":      true,
	"construction:amenity": true,
	"leisure":              true,
	"tourism":              true,
	"natural":              true,
	"generator:source":     true,
}

func main() {

	batchSize := 1000
	collectionName := "nodes"

	db, err := mongodb.New(context.Background(), os.Getenv("MONGODB_URI"), os.Getenv("MONGODB_DATABASE"))
	if err != nil {
		log.Fatal(err)
	}
	_ = db

	f, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	dryRun := false
	for _, arg := range os.Args {
		if arg == "--dryRun" {
			dryRun = true
		}
	}

	bar := progressbar.Default(-1)

	scanner := osmpbf.New(context.Background(), f, 1)
	defer scanner.Close()

	scanner.SkipWays = true
	scanner.FilterNode = func(n *osm.Node) bool {
		for _, t := range n.Tags {
			if mainTags[t.Key] {
				return true
			}
		}
		return false
	}

	features := make([]*geojson.Feature, 0, batchSize)

	countByTag := make(TagCounts)

	c := 0
	for scanner.Scan() {
		node, ok := scanner.Object().(*osm.Node)
		if !ok {
			continue
		}

		for _, t := range node.Tags {
			countByTag[t.Key] += 1
		}
		c += 1
		bar.Add(1)
		if !dryRun {
			features = append(features, convert(node))
			if len(features) >= batchSize {
				if err := db.Insert(context.Background(), collectionName, features); err != nil {
					log.Fatal(err)
				}

				features = features[:0]
			}
		}
	}

	if len(features) > 0 {
		if err := db.Insert(context.Background(), collectionName, features); err != nil {
			log.Fatal(err)
		}
	}

	log.Println(countByTag.Counts(10))
	log.Println("imported features: ", c)
}

type TagCounts map[string]int

func (m TagCounts) Counts(minCount int) []TagCount {
	c := make(ByCount, 0, len(m))
	for k, v := range m {
		if v >= minCount {
			c = append(c, TagCount{Tag: k, Count: v})
		}
	}
	sort.Sort(c)
	return c
}

type TagCount struct {
	Tag   string
	Count int
}
type ByCount []TagCount

func (l ByCount) Len() int {
	return len(l)
}

func (l ByCount) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}
func (l ByCount) Less(i, j int) bool {
	return l[i].Count < l[j].Count
}

func convert(node *osm.Node) *geojson.Feature {

	f := geojson.NewFeature(node.Point())
	f.ID = node.ID
	f.Properties["id"] = node.ID
	f.Properties["type"] = "node"
	if !node.Timestamp.IsZero() {
		f.Properties["timestamp"] = node.Timestamp
	}
	if node.Version != 0 {
		f.Properties["version"] = node.Version
	}
	if node.ChangesetID != 0 {
		f.Properties["changeset"] = node.ChangesetID
	}
	if node.User != "" {
		f.Properties["user"] = node.User
	}
	if node.ChangesetID != 0 {
		f.Properties["uid"] = node.UserID
	}
	f.Properties["tags"] = node.Tags.Map()

	return f
}
