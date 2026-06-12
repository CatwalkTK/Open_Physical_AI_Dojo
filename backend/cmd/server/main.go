package main

import (
	"log"
	"os"

	"open-physical-ai-dojo/backend/internal/api"
	"open-physical-ai-dojo/backend/internal/integration/perception"
	"open-physical-ai-dojo/backend/internal/integration/robot"
	"open-physical-ai-dojo/backend/internal/repository"
	"open-physical-ai-dojo/backend/internal/service"
)

func main() {
	dogzillaURL := getenv("DOGZILLA_RUNTIME_URL", "http://localhost:8090")
	perceptionURL := getenv("PERCEPTION_SERVICE_URL", "http://localhost:8070")
	dataDir := getenv("DATA_DIR", "../data")
	port := getenv("PORT", "8080")

	store, err := repository.NewJSONLStore(dataDir)
	if err != nil {
		log.Fatal(err)
	}
	taskService := service.NewTaskService(
		robot.NewDogzillaClient(dogzillaURL),
		perception.NewClient(perceptionURL),
		store,
	)
	lessonService := service.NewLessonService(store)
	router := api.NewRouter(taskService, lessonService)

	log.Printf(
		"backend listening on :%s, dogzilla runtime: %s, perception service: %s, data dir: %s",
		port,
		dogzillaURL,
		perceptionURL,
		dataDir,
	)
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
