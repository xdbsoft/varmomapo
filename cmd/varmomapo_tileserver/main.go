package main

import (
	"context"
	"embed"
	"log"
	"net/http"
	"os"
	"text/template"

	"github.com/xdbsoft/varmomapo/config"
	"github.com/xdbsoft/varmomapo/mongodb"
	"github.com/xdbsoft/varmomapo/tileserver"
)

//go:embed config.yaml
var configContent []byte

//go:embed template/*
var f embed.FS

func main() {

	cfg, err := config.ParseYAML(configContent)
	if err != nil {
		log.Fatal(err)
	}

	db, err := mongodb.New(context.Background(), os.Getenv("MONGODB_URI"), os.Getenv("MONGODB_DATABASE"))
	if err != nil {
		log.Fatal(err)
	}

	s := tileserver.Server{
		Config: *cfg,
		DB:     db,
	}

	log.Print("starting server...")
	http.HandleFunc("/tiles/", s.TilesHandler)
	http.HandleFunc("/", pageHandler(*cfg))

	// Determine port for HTTP service.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("defaulting to port %s", port)
	}

	// Start HTTP server.
	log.Printf("listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func pageHandler(cfg config.Config) func(w http.ResponseWriter, r *http.Request) {
	tpl := template.Must(template.ParseFS(f, "template/layout.html"))

	return func(w http.ResponseWriter, r *http.Request) {
		if err := tpl.Execute(w, cfg); err != nil {
			log.Print(err)
		}
	}
}
