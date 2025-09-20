package main

import (
	"flag"
	"log"
	"os"

	"github.com/df07/scene-llm/web/server"
)

func main() {
	// Parse command line flags
	port := flag.Int("port", 8081, "Port to serve on")
	flag.Parse()

	// Create and start web server
	webServer := server.NewServer(*port)

	log.Printf("Scene LLM Web Server")
	log.Printf("Visit http://localhost:%d to start creating scenes", *port)

	if err := webServer.Start(); err != nil {
		log.Printf("Error starting server: %v", err)
		os.Exit(1)
	}
}
