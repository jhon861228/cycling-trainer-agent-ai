package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"cycling-trainer-agent-ai/backend/ai"
	"cycling-trainer-agent-ai/backend/auth"
	"cycling-trainer-agent-ai/backend/models"
	"cycling-trainer-agent-ai/backend/workouts"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var dbClient *dynamodb.Client
var tableName string
var rateLimiter *ai.RateLimiter

func setupDB() {
	fmt.Println(">> 1. Verificando Variables de Entorno...")
	endpoint := os.Getenv("DYNAMODB_ENDPOINT")
	tableName = os.Getenv("DYNAMODB_TABLE")
	apiKey := os.Getenv("GEMINI_API_KEY")
	port := os.Getenv("PORT")

	fmt.Printf("   - DYNAMODB_ENDPOINT: %s\n", endpoint)
	fmt.Printf("   - DYNAMODB_TABLE:    %s\n", tableName)
	fmt.Printf("   - GEMINI_API_KEY:    %s\n", maskKey(apiKey))
	fmt.Printf("   - PORT:              %s\n", port)

	if tableName == "" {
		tableName = "CyclingTrainerData"
	}

	fmt.Println(">> 2. Cargando configuración de AWS...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Cargar configuración base
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		fmt.Printf("!! Error cargando AWS config: %v\n", err)
	}

	// Crear cliente de DynamoDB con el endpoint si está definido
	if endpoint != "" {
		fmt.Printf(">> 3. Configurando cliente con endpoint local: %s\n", endpoint)
		dbClient = dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
			o.BaseEndpoint = aws.String(endpoint)
		})
	} else {
		fmt.Println(">> 3. Usando endpoint por defecto de AWS (Cloud)")
		dbClient = dynamodb.NewFromConfig(cfg)
	}

	fmt.Printf(">> 4. Base de datos lista. Tabla: %s\n", tableName)
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "...." + key[len(key)-4:]
}

type AuthRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	fmt.Println(">> Procesando Login...")
	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fmt.Printf("!! Error decodificando login: %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	token, err := auth.Login(r.Context(), dbClient, tableName, req.Email, req.Password)
	if err != nil {
		fmt.Printf("!! Error en login para %s: %v\n", req.Email, err)
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	fmt.Printf(">> Login exitoso para %s\n", req.Email)
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	fmt.Println(">> Procesando Registro...")
	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fmt.Printf("!! Error decodificando registro: %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := auth.Register(r.Context(), dbClient, tableName, req.Email, req.Password)
	if err != nil {
		fmt.Printf("!! Error en registro para %s: %v\n", req.Email, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Printf(">> Registro exitoso para %s\n", req.Email)
	w.WriteHeader(http.StatusCreated)
}

func handleGeneratePlan(w http.ResponseWriter, r *http.Request) {
	fmt.Println(">> Generando Plan de Entrenamiento con IA...")
	var req struct {
		Goal         string         `json:"goal"`
		FTP          float64        `json:"ftp"`
		Weight       float64        `json:"weight"`
		Email        string         `json:"email"`
		Days         int            `json:"days"`
		Availability map[string]int `json:"availability"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fmt.Printf("!! Error decodificando petición de plan: %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validar email
	if req.Email == "" {
		http.Error(w, "Email es requerido", http.StatusBadRequest)
		return
	}

	// Validar días (default 7, min 1, max 30)
	if req.Days <= 0 {
		req.Days = 7
	}
	if req.Days > 30 {
		req.Days = 30
	}

	// Validar longitud del texto
	if err := ai.ValidateGoalInput(req.Goal); err != nil {
		fmt.Printf("!! Validación fallida: %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Verificar límite de uso
	if err := rateLimiter.CheckLimit(req.Email); err != nil {
		fmt.Printf("!! Rate limit excedido para %s\n", req.Email)
		http.Error(w, err.Error(), http.StatusTooManyRequests)
		return
	}

	// Generar entrenamientos estructurados con IA
	dayWorkouts, title, err := ai.GenerateWorkouts(r.Context(), req.Goal, req.FTP, req.Weight, req.Days, req.Availability)
	if err != nil {
		fmt.Printf("!! Error de IA (Gemini): %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Generar ID del plan y timestamp
	planID := fmt.Sprintf("%d", time.Now().UnixMilli())
	createdAt := time.Now().Format(time.RFC3339)

	// 1. Marcar plan antiguo como inactivo
	pk := fmt.Sprintf("USER#%s", req.Email)
	fmt.Printf(">> Moviendo plan anterior a historial para %s...\n", req.Email)

	activeRecords, _ := dbClient.Query(r.Context(), &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
		FilterExpression:       aws.String("is_active = :active"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":     &types.AttributeValueMemberS{Value: pk},
			":sk":     &types.AttributeValueMemberS{Value: "DAY#"},
			":active": &types.AttributeValueMemberBOOL{Value: true},
		},
	})

	if activeRecords != nil {
		for _, item := range activeRecords.Items {
			dbClient.UpdateItem(r.Context(), &dynamodb.UpdateItemInput{
				TableName: aws.String(tableName),
				Key: map[string]types.AttributeValue{
					"PK": item["PK"],
					"SK": item["SK"],
				},
				UpdateExpression: aws.String("SET is_active = :inactive"),
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":inactive": &types.AttributeValueMemberBOOL{Value: false},
				},
			})
		}
		fmt.Printf("   - %d días movidos a historial.\n", len(activeRecords.Items))
	}

	// 2. Convertir a ZWO y guardar en DynamoDB
	var savedWorkouts []models.WorkoutDay
	for _, dw := range dayWorkouts {
		zwoContent := ""
		if dw.Type != "rest" && len(dw.Intervals) > 0 {
			zwoContent = workouts.GenerateZWOFromWorkout(dw, req.FTP)
		}

		workoutDay := models.WorkoutDay{
			PK:        pk,
			SK:        fmt.Sprintf("DAY#%d#%s", dw.Day, planID), // ID único para evitar colisiones en historial
			PlanID:    planID,
			IsActive:  true,
			Day:       dw.Day,
			Name:      dw.Name,
			Type:      dw.Type,
			Duration:  dw.Duration,
			Intervals: dw.Intervals,
			ZWO:       zwoContent,
			Goal:      req.Goal,
			Title:     title,
			CreatedAt: createdAt,
		}

		// Guardar en DynamoDB
		item, err := attributevalue.MarshalMap(workoutDay)
		if err != nil {
			fmt.Printf("!! Error serializando día %d: %v\n", dw.Day, err)
			continue
		}

		_, err = dbClient.PutItem(r.Context(), &dynamodb.PutItemInput{
			TableName: aws.String(tableName),
			Item:      item,
		})
		if err != nil {
			fmt.Printf("!! Error guardando día %d en DynamoDB: %v\n", dw.Day, err)
			continue
		}

		savedWorkouts = append(savedWorkouts, workoutDay)
	}

	fmt.Printf(">> Plan guardado: %d días para %s (ID: %s)\n", len(savedWorkouts), req.Email, planID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"plan_id":  planID,
		"workouts": savedWorkouts,
	})
}

func handleGetWorkouts(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	history := r.URL.Query().Get("history") == "true"

	if email == "" {
		http.Error(w, "email es requerido", http.StatusBadRequest)
		return
	}

	fmt.Printf(">> Consultando entrenamientos para %s (Historial: %v)\n", email, history)

	pk := fmt.Sprintf("USER#%s", email)
	skPrefix := "DAY#"

	input := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: pk},
			":sk": &types.AttributeValueMemberS{Value: skPrefix},
		},
	}

	if !history {
		input.FilterExpression = aws.String("is_active = :active")
		input.ExpressionAttributeValues[":active"] = &types.AttributeValueMemberBOOL{Value: true}
	} else {
		input.FilterExpression = aws.String("is_active = :active")
		input.ExpressionAttributeValues[":active"] = &types.AttributeValueMemberBOOL{Value: false}
	}
	result, err := dbClient.Query(r.Context(), input)
	if err != nil {
		fmt.Printf("!! Error consultando workouts: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var wkouts []models.WorkoutDay
	err = attributevalue.UnmarshalListOfMaps(result.Items, &wkouts)
	if err != nil {
		fmt.Printf("!! Error deserializando workouts: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wkouts)
}

func handleDownloadWorkout(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	planID := r.URL.Query().Get("plan_id")
	day := r.URL.Query().Get("day")

	if email == "" || planID == "" || day == "" {
		http.Error(w, "email, plan_id y day son requeridos", http.StatusBadRequest)
		return
	}

	pk := fmt.Sprintf("USER#%s", email)
	sk := fmt.Sprintf("DAY#%s#%s", day, planID)

	result, err := dbClient.GetItem(r.Context(), &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
	})
	if err != nil || result.Item == nil {
		http.Error(w, "Entrenamiento no encontrado", http.StatusNotFound)
		return
	}

	var workout models.WorkoutDay
	attributevalue.UnmarshalMap(result.Item, &workout)

	if workout.ZWO == "" {
		http.Error(w, "Este día es de descanso, no tiene archivo ZWO", http.StatusNotFound)
		return
	}

	filename := fmt.Sprintf("dia_%s_%s.zwo", day, workout.Type)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Write([]byte(workout.ZWO))
}

func handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	fmt.Println(">> Actualizando Perfil...")
	var req struct {
		Email  string  `json:"email"`
		FTP    float64 `json:"ftp"`
		Weight float64 `json:"weight"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fmt.Printf("!! Error decodificando actualización de perfil: %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := auth.UpdateProfile(r.Context(), dbClient, tableName, req.Email, req.FTP, req.Weight)
	if err != nil {
		fmt.Printf("!! Error en DB actualizando perfil: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Printf(">> Perfil actualizado para %s\n", req.Email)
	w.WriteHeader(http.StatusOK)
}

func handleGetProfile(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	fmt.Printf(">> Consultando Perfil para %s...\n", email)
	if email == "" {
		http.Error(w, "email is required", http.StatusBadRequest)
		return
	}

	user, err := auth.GetProfile(r.Context(), dbClient, tableName, email)
	if err != nil {
		fmt.Printf("!! Error obteniendo perfil de %s: %v\n", email, err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(user)
}

func middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Solicitud: %s %s", r.Method, r.URL.Path)
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func handlePing(w http.ResponseWriter, r *http.Request) {
	fmt.Println(">> Recibido PING")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("pong"))
}

func main() {
	fmt.Println("========================================")
	fmt.Println("   Servidor CyclingAI Iniciando...      ")
	fmt.Println("========================================")

	setupDB()
	rateLimiter = ai.NewRateLimiter()

	mux := http.NewServeMux()
	mux.HandleFunc("/ping", handlePing)
	mux.HandleFunc("/login", handleLogin)
	mux.HandleFunc("/register", handleRegister)
	mux.HandleFunc("/generate-plan", handleGeneratePlan)
	mux.HandleFunc("/download-workout", handleDownloadWorkout)
	mux.HandleFunc("/workouts", handleGetWorkouts)
	mux.HandleFunc("/update-profile", handleUpdateProfile)
	mux.HandleFunc("/profile", handleGetProfile)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf(">> 4. Servidor listo en http://localhost:%s\n", port)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: middleware(mux),
	}

	err := server.ListenAndServe()
	if err != nil {
		log.Fatalf("Error fatal al iniciar servidor: %v", err)
	}
}
