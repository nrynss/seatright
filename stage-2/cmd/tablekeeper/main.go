package main

import (
	"log"
	"net/http"
	"os"

	"tablekeeper/internal/service"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	srv := service.New()
	log.Printf("tablekeeper listening on 0.0.0.0:%s", port)
	if err := http.ListenAndServe("0.0.0.0:"+port, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}
