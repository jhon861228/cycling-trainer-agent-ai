package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"cycling-trainer-agent-ai/backend/ai"
	"cycling-trainer-agent-ai/backend/auth"
	"cycling-trainer-agent-ai/backend/handlers"
	"cycling-trainer-agent-ai/backend/repository"

	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("   Servidor CyclingAI (Refactored)      ")
	fmt.Println("========================================")

	// 1. Setup Configuration
	endpoint := os.Getenv("DYNAMODB_ENDPOINT")
	tableName := os.Getenv("DYNAMODB_TABLE")
	apiKey := os.Getenv("GEMINI_API_KEY")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if tableName == "" {
		tableName = "CyclingTrainerData"
	}

	// 2. Setup AWS / DynamoDB Client
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("!! Error loading AWS config: %v", err)
	}

	var dbClient *dynamodb.Client
	if endpoint != "" {
		dbClient = dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
			o.BaseEndpoint = aws.String(endpoint)
		})
	} else {
		dbClient = dynamodb.NewFromConfig(cfg)
	}

	// 3. Dependency Injection
	repo := repository.NewDynamoRepository(dbClient, tableName)
	authSvc := auth.NewService(repo)
	aiSvc := ai.NewService(apiKey)
	h := handlers.NewHandler(authSvc, aiSvc, repo)

	// 4. Routing
	mux := http.NewServeMux()

	// API Endpoints
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})

	mux.HandleFunc("/login", h.HandleLogin)
	mux.HandleFunc("/register", h.HandleRegister)
	mux.HandleFunc("/profile", h.HandleGetProfile)
	mux.HandleFunc("/update-profile", h.HandleUpdateProfile)
	mux.HandleFunc("/generate-plan", h.HandleGeneratePlan)
	mux.HandleFunc("/workouts", h.HandleGetWorkouts)
	mux.HandleFunc("/download-workout", h.HandleDownloadWorkout)
	mux.HandleFunc("/toggle-workout", h.HandleToggleWorkoutDone)
	// (Check if HandleDownloadWorkout is needed, assuming yes)
	// mux.HandleFunc("/download-workout", h.HandleDownloadWorkout) // Add this to handlers if needed

	// 5. Middleware & Start
	fmt.Printf(">> Servidor listo en http://localhost:%s\n", port)
	server := &http.Server{
		Addr:    ":" + port,
		Handler: middleware(mux),
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Error fatal al iniciar servidor: %v", err)
	}
}

func middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		// Set CORS headers
		wrapped.Header().Set("Access-Control-Allow-Origin", "*")
		wrapped.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		wrapped.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			wrapped.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(wrapped, r)

		log.Printf("[%s] %s %s %d (%v)",
			r.RemoteAddr,
			r.Method,
			r.URL.Path,
			wrapped.status,
			time.Since(start),
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}
