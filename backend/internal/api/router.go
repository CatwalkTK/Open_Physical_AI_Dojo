package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"open-physical-ai-dojo/backend/internal/domain"
	"open-physical-ai-dojo/backend/internal/service"
)

func NewRouter(taskService *service.TaskService) *gin.Engine {
	router := gin.Default()
	router.Use(cors())

	api := router.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})
		api.POST("/tasks", createTask(taskService))
		api.GET("/tasks/:id", getTask(taskService))
		api.POST("/perception", runPerception(taskService))
		api.GET("/perception/status", getPerceptionStatus(taskService))
		api.GET("/robot/dogzilla", getDogzillaStatus(taskService))
		api.POST("/robot/dogzilla/stop", emergencyStopDogzilla(taskService))
		api.POST("/plans", createPlan(taskService))
		api.POST("/actions/execute", executeTask(taskService))
		api.POST("/actions/stop", stopTask(taskService))
		api.POST("/evaluations/run", runEvaluation(taskService))
		api.GET("/evaluations", listEvaluations(taskService))
		api.GET("/episodes", listEpisodes(taskService))
		api.GET("/stream/tasks/:id", streamTask(taskService))
	}

	return router
}

func getPerceptionStatus(taskService *service.TaskService) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := taskService.PerceptionStatus()
		if !status.Connected {
			c.JSON(http.StatusServiceUnavailable, status)
			return
		}
		c.JSON(http.StatusOK, status)
	}
}

func runEvaluation(taskService *service.TaskService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req domain.EvaluationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, taskService.RunEvaluation(req))
	}
}

func getDogzillaStatus(taskService *service.TaskService) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := taskService.DogzillaStatus()
		if !status.Connected {
			c.JSON(http.StatusServiceUnavailable, status)
			return
		}
		c.JSON(http.StatusOK, status)
	}
}

func emergencyStopDogzilla(taskService *service.TaskService) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := taskService.EmergencyStopDogzilla()
		if status.Error != "" {
			c.JSON(http.StatusServiceUnavailable, status)
			return
		}
		c.JSON(http.StatusOK, status)
	}
}

func runPerception(taskService *service.TaskService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req domain.PerceptionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, taskService.RunPerception(req))
	}
}

func createTask(taskService *service.TaskService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req domain.CreateTaskRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		task, err := taskService.CreateTask(req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, task)
	}
}

func getTask(taskService *service.TaskService) gin.HandlerFunc {
	return func(c *gin.Context) {
		task, ok := taskService.GetTask(c.Param("id"))
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		c.JSON(http.StatusOK, task)
	}
}

func createPlan(taskService *service.TaskService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req domain.PlanRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, taskService.GeneratePlan(req))
	}
}

func executeTask(taskService *service.TaskService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req domain.ExecuteActionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		task, err := taskService.Execute(req.TaskID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusAccepted, task)
	}
}

func stopTask(taskService *service.TaskService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req domain.StopActionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		task, err := taskService.Stop(req.TaskID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, task)
	}
}

func listEpisodes(taskService *service.TaskService) gin.HandlerFunc {
	return func(c *gin.Context) {
		episodes, err := taskService.ListPersistedEpisodes(20)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, episodes)
	}
}

func listEvaluations(taskService *service.TaskService) gin.HandlerFunc {
	return func(c *gin.Context) {
		evaluations, err := taskService.ListPersistedEvaluations(20)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, evaluations)
	}
}

func streamTask(taskService *service.TaskService) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")
		ch := taskService.Stream().Subscribe(taskID)
		defer taskService.Stream().Unsubscribe(taskID, ch)

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")

		if task, ok := taskService.GetTask(taskID); ok {
			c.SSEvent("task", task)
			c.Writer.Flush()
		}

		for {
			select {
			case <-c.Request.Context().Done():
				return
			case message := <-ch:
				c.SSEvent("task", message)
				c.Writer.Flush()
			}
		}
	}
}

func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
