package main

import (
	"log"
	"os"

	"open-physical-ai-dojo/backend/internal/api"
	"open-physical-ai-dojo/backend/internal/integration/robot"
	"open-physical-ai-dojo/backend/internal/repository"
	"open-physical-ai-dojo/backend/internal/service"
)

func main() {
	dogzillaURL := getenv("DOGZILLA_RUNTIME_URL", "http://localhost:8090")
	dataDir := getenv("DATA_DIR", "../data")
	port := getenv("PORT", "8080")

	store, err := repository.NewJSONLStore(dataDir)
	if err != nil {
		log.Fatal(err)
	}
	taskService := service.NewTaskService(robot.NewDogzillaClient(dogzillaURL), store)
	router := api.NewRouter(taskService)

	log.Printf("backend listening on :%s, dogzilla runtime: %s, data dir: %s", port, dogzillaURL, dataDir)
	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
