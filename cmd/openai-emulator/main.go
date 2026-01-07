package main

import (
	"log"
	"os"

	"github.com/fabianvf/llemulator/internal/server"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := server.NewServer()
	log.Fatal(srv.Run(port))
}
