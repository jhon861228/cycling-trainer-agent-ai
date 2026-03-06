package handlers

import (
	"cycling-trainer-agent-ai/backend/ai"
	"cycling-trainer-agent-ai/backend/auth"
	"cycling-trainer-agent-ai/backend/models"
	"cycling-trainer-agent-ai/backend/repository"
	"cycling-trainer-agent-ai/backend/workouts"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type Handler struct {
	authSvc auth.Service
	aiSvc   ai.Service
	repo    repository.Repository
}

func NewHandler(authSvc auth.Service, aiSvc ai.Service, repo repository.Repository) *Handler {
	return &Handler{
		authSvc: authSvc,
		aiSvc:   aiSvc,
		repo:    repo,
	}
}

func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	token, err := h.authSvc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func (h *Handler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email     string `json:"email"`
		Password  string `json:"password"`
		FirstName string `json:"first_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.authSvc.Register(r.Context(), req.Email, req.Password, req.FirstName); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) HandleGetProfile(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		http.Error(w, "email is required", http.StatusBadRequest)
		return
	}

	user, err := h.authSvc.GetProfile(r.Context(), email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(user)
}

func (h *Handler) HandleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email  string  `json:"email"`
		FTP    float64 `json:"ftp"`
		Weight float64 `json:"weight"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.authSvc.UpdateProfile(r.Context(), req.Email, req.FTP, req.Weight); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) HandleGeneratePlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email        string         `json:"email"`
		Goal         string         `json:"goal"`
		Days         int            `json:"days"`
		Availability map[string]int `json:"availability"`
		FTP          float64        `json:"ftp"`
		Weight       float64        `json:"weight"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Email == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}

	// 1. Resolve Athlete Name
	user, err := h.authSvc.GetProfile(r.Context(), req.Email)
	userName := "Atleta"
	if err == nil && user != nil && user.FirstName != "" {
		userName = user.FirstName
	}

	// 2. Generate Workouts via AI
	log.Printf("[AI] Generating plan for %s (Goal: %s, Days: %d, FTP: %.0f)", req.Email, req.Goal, req.Days, req.FTP)
	workoutsList, title, err := h.aiSvc.GenerateWorkouts(r.Context(), req.Goal, req.FTP, req.Weight, req.Days, req.Availability, userName)
	if err != nil {
		log.Printf("[ERROR] AI Generation failed: %v", err)
		http.Error(w, "Error generating workouts: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. Deactivate current active plan
	if err := h.repo.DeactivateActiveWorkouts(r.Context(), req.Email); err != nil {
		log.Printf("[WARNING] Could not deactivate old workouts: %v", err)
	}

	planID := fmt.Sprintf("%d", time.Now().Unix())
	createdAt := time.Now().Format(time.RFC3339)

	// 4. Transform and Save each day
	for _, dw := range workoutsList {
		// Generate ZWO if not a rest day
		var zwoContent string
		if dw.Type != "rest" {
			zwoContent = workouts.GenerateZWOFromWorkout(dw, req.FTP)
		}

		record := models.WorkoutDay{
			PK:        "USER#" + req.Email,
			SK:        fmt.Sprintf("DAY#%d#%s", dw.Day, planID),
			PlanID:    planID,
			IsActive:  true,
			Day:       dw.Day,
			Name:      dw.Name,
			Title:     title,
			Type:      dw.Type,
			Duration:  dw.Duration,
			Intervals: dw.Intervals,
			ZWO:       zwoContent,
			Goal:      req.Goal,
			CreatedAt: createdAt,
		}

		if err := h.repo.SaveWorkoutDay(r.Context(), &record); err != nil {
			log.Printf("[ERROR] Failed to save day %d: %v", dw.Day, err)
		}
	}

	log.Printf("[SUCCESS] Plan %s generated with %d days", planID, len(workoutsList))
	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) HandleGetWorkouts(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	history := r.URL.Query().Get("history") == "true"

	workouts, err := h.repo.GetWorkouts(r.Context(), email, !history)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(workouts)
}

func (h *Handler) HandleDownloadWorkout(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	planID := r.URL.Query().Get("plan_id")
	day := r.URL.Query().Get("day")

	if email == "" || planID == "" || day == "" {
		http.Error(w, "email, plan_id y day son requeridos", http.StatusBadRequest)
		return
	}

	workout, err := h.repo.GetWorkout(r.Context(), email, planID, day)
	if err != nil || workout == nil {
		http.Error(w, "Entrenamiento no encontrado", http.StatusNotFound)
		return
	}

	if workout.ZWO == "" {
		http.Error(w, "Este día es de descanso, no tiene archivo ZWO", http.StatusNotFound)
		return
	}

	filename := fmt.Sprintf("dia_%s_%s.zwo", day, workout.Type)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Write([]byte(workout.ZWO))
}

func (h *Handler) HandleToggleWorkoutDone(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email     string `json:"email"`
		PlanID    string `json:"plan_id"`
		Day       string `json:"day"`
		Completed bool   `json:"completed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Error decodificando el cuerpo: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Email == "" || req.PlanID == "" || req.Day == "" {
		http.Error(w, "email, plan_id y day son requeridos", http.StatusBadRequest)
		return
	}

	if err := h.repo.ToggleWorkoutDone(r.Context(), req.Email, req.PlanID, req.Day, req.Completed); err != nil {
		http.Error(w, "Error actualizando el estado del entrenamiento: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
