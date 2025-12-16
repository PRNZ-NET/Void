package main

import (
	"Void/internal/server"
	"flag"
	"log"
)

func main() {
	port := flag.String("port", "8080", "Server port")
	flag.Parse()

	s := server.NewServer(*port)
	log.Printf("Starting Void server on port %s", *port)
	if err := s.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

