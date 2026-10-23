package main

import (
	"log"
	"net/http"
	"time"
)

func main() {
	router := http.NewServeMux()
	router.HandleFunc("GET /health/live", func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		_, writeError := responseWriter.Write([]byte(`{"status":"running"}`))
		if writeError != nil {
			log.Printf("Erro ao escrever resposta: %v", writeError)
		}
	})
	server := &http.Server{
		Addr:              ":8080",
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Println("Servidor: http://localhost:8080/health/live")
	log.Fatal(server.ListenAndServe())
}
