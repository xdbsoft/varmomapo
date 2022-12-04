package tileserver

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/dustin/go-heatmap/schemes"
	"github.com/paulmach/orb/maptile"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/xdbsoft/varmomapo/config"
	"github.com/xdbsoft/varmomapo/heatmap"
	"github.com/xdbsoft/varmomapo/mongodb"
)

type Server struct {
	Config config.Config
	DB     *mongodb.Client
}

var urlRegex = regexp.MustCompile(`\A/.*/(\w+)/(\d+)/(\d+)/(\d+)\.(png|jpeg)\z`)

var empty []byte

func init() {
	img := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	for x := 0; x < 256; x++ {
		for y := 0; y < 256; y++ {
			img.Set(x, y, color.Transparent)
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		log.Fatal(err)
	}
	empty = buffer.Bytes()
}

func (s *Server) TilesHandler(w http.ResponseWriter, r *http.Request) {
	//Split URL
	m := urlRegex.FindStringSubmatch(r.URL.Path)
	if m == nil || len(m) != 6 {
		log.Print("Invalid URL: ", r.URL.Path, m)
		http.NotFound(w, r)
		return
	}

	layerName := m[1]

	//Decode layer
	filter, found := s.Config.FilterByLayer[layerName]
	if !found {
		log.Println("Invalid layer", layerName)
		http.NotFound(w, r)
		return
	}

	//Decode level, x and y
	level, err := strconv.Atoi(m[2])
	if err != nil {
		log.Print("Error decoding level: ", m[2])
		http.NotFound(w, r)
		return
	}

	x, err := strconv.Atoi(m[3])
	if err != nil {
		log.Print("Error decoding x: ", m[3])
		http.NotFound(w, r)
		return
	}
	y, err := strconv.Atoi(m[4])
	if err != nil {
		log.Print("Error decoding y: ", m[4])
		http.NotFound(w, r)
		return
	}

	cached := s.getCached(r.Context(), layerName, level, x, y)
	if cached != nil {
		log.Println(layerName, level, x, y, "found in cache")
		w.Write(cached)
		return
	}

	tile := maptile.New(uint32(x), uint32(y), maptile.Zoom(level))

	tileData, err := s.getRaw(r.Context(), tile, filter)

	if err != nil {
		log.Print("Error: ", level, x, y, err)
		http.NotFound(w, r)
		return
	}

	s.putInCache(r.Context(), layerName, level, x, y, tileData)

	w.Write(tileData)
}

type cacheItem struct {
	CreationDate time.Time
	Hit          int
	Layer        string
	Level        int
	X            int
	Y            int
	Data         []byte
}

func (s *Server) getCached(ctx context.Context, layerName string, level int, x int, y int) []byte {

	var item cacheItem
	filter := bson.D{
		{Key: "layer", Value: layerName},
		{Key: "level", Value: level},
		{Key: "x", Value: x},
		{Key: "y", Value: y},
	}
	err := s.DB.Get(ctx, "cache", filter, &item)
	if err != nil {
		return nil
	}
	s.DB.Inc(ctx, "cache", filter, "hit")
	return item.Data
}

func (s *Server) putInCache(ctx context.Context, layerName string, level int, x int, y int, data []byte) {
	item := cacheItem{
		CreationDate: time.Now(),
		Hit:          1,
		Layer:        layerName,
		Level:        level,
		X:            x,
		Y:            y,
		Data:         data,
	}

	if err := s.DB.Put(ctx, "cache", item); err != nil {
		log.Println("Failed to put in cache", err)
	}
}

func (s *Server) getRaw(ctx context.Context, tile maptile.Tile, filter *bson.D) ([]byte, error) {
	features, err := s.DB.FindInBBox(ctx, "nodes", tile.Bound(16./256.), filter)
	if err != nil {
		log.Print("Error: ", tile, err)
		return nil, err
	}
	log.Println("Features in", tile, ":", len(features))

	if len(features) == 0 {
		return empty, nil
	}

	// 256 is 2^8, thus projecting 8 levels further than tile should give us the pixel
	tLeftTop := maptile.At(tile.Bound().LeftTop(), tile.Z+8)

	points := []image.Point{}
	for _, f := range features {
		t := maptile.At(f.Point(), tile.Z+8)

		points = append(points, image.Point{
			X: int(t.X) - int(tLeftTop.X),
			Y: 256 - (int(t.Y) - int(tLeftTop.Y)),
		})
	}

	dotRadius := 16
	imageSize := image.Rect(0, 0, 256, 256)
	limits := imageSize.Inset(-dotRadius)

	imgFull := heatmap.Heatmap(limits.Add(image.Pt(dotRadius, dotRadius)), points, limits, dotRadius*2, 128, schemes.PBJ)

	img := imgFull.SubImage(imageSize.Add(image.Pt(dotRadius, dotRadius)))

	b := bytes.Buffer{}
	if err := png.Encode(&b, img); err != nil {
		log.Fatal(err)
	}

	return b.Bytes(), nil
}
