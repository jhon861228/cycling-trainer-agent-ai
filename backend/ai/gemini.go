package ai

import (
	"context"
	"cycling-trainer-agent-ai/backend/models"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// Service defines the business logic for AI content generation
type Service interface {
	GenerateWorkouts(ctx context.Context, goal string, ftp float64, weight float64, days int, availability map[string]int, userName string) ([]models.DayWorkout, string, error)
}

type aiService struct {
	apiKey string
}

// NewService creates a new AI service
func NewService(apiKey string) Service {
	return &aiService{
		apiKey: apiKey,
	}
}

func (s *aiService) GenerateWorkouts(ctx context.Context, goal string, ftp float64, weight float64, days int, availability map[string]int, userName string) ([]models.DayWorkout, string, error) {
	if os.Getenv("MOCK_AI") == "true" {
		log.Printf("[AI] Running in MOCK_AI mode for athlete: %s", userName)
		workouts, title := generateMockWorkouts(ftp, days, availability, userName)
		return workouts, title, nil
	}

	if s.apiKey == "" {
		log.Printf("[ERROR] AI generation failed: GEMINI_API_KEY is not set")
		return nil, "", fmt.Errorf("GEMINI_API_KEY is not set")
	}

	log.Printf("[AI] Sending generation request to Gemini for athlete: %s", userName)

	client, err := genai.NewClient(ctx, option.WithAPIKey(s.apiKey))
	if err != nil {
		return nil, "", fmt.Errorf("creating GenAI client: %w", err)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-2.5-flash")
	model.ResponseMIMEType = "application/json"

	availStr, _ := json.Marshal(availability)

	prompt := fmt.Sprintf(
		"Actúa como un entrenador de ciclismo experto. Genera un plan de entrenamiento de exactamente %d días para un ciclista llamado %s con un objetivo de: '%s'. "+
			"Disponibilidad semanal (minutos por día): %s. "+
			"Reglas CRÍTICAS de Formato JSON: "+
			"1. Genera un TÍTULO corto y motivador (máximo 5 palabras). "+
			"2. El resultado DEBE ser un objeto JSON válido con este formato: "+
			"{ \"title\": \"...\", \"workouts\": [ { \"day\": 1, \"name\": \"...\", \"type\": \"...\", \"duration_minutes\": ..., \"intervals\": [...] } ] }. "+
			"3. IMPORTANTE: Cada elemento del array 'intervals' DEBE ser un OBJETO con estos campos: "+
			"   - \"type\": string (\"warmup\", \"cooldown\", \"steady\", \"interval\") "+
			"   - \"duration\": int (duración en SEGUNDOS) "+
			"   - \"power\": float (porcentaje de FTP, ej: 0.75 para 75%%) "+
			"   - Si el tipo es \"interval\", incluye: \"on_duration\", \"off_duration\" (segundos), \"on_power\", \"off_power\" (float) y \"repeat\" (int). "+
			"4. Cada nombre de entrenamiento debe empezar con '%s - '. "+
			"5. La suma de 'duration' de los intervalos debe coincidir con 'duration_minutes' (convertido a segundos). "+
			"6. El FTP del usuario es %.0f W y su peso es %.1f kg. "+
			"Genera EXACTAMENTE %d días. Regresa SOLAMENTE JSON puro.",
		days, userName, goal, string(availStr), userName, ftp, weight, days,
	)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, "", fmt.Errorf("generating content: %w", err)
	}

	if len(resp.Candidates) == 0 {
		return nil, "", fmt.Errorf("no candidates in response")
	}

	var responseText strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		responseText.WriteString(fmt.Sprintf("%v", part))
	}

	cleanJSON := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(responseText.String(), "```json"), "```"))

	var aiResponse struct {
		Title    string              `json:"title"`
		Workouts []models.DayWorkout `json:"workouts"`
	}
	if err := json.Unmarshal([]byte(cleanJSON), &aiResponse); err != nil {
		log.Printf("[ERROR] Failed to unmarshal AI response. Raw content: %s", cleanJSON)
		return nil, "", fmt.Errorf("unmarshaling AI response: %w", err)
	}

	log.Printf("[AI] Successfully generated %d days for %s", len(aiResponse.Workouts), userName)
	return aiResponse.Workouts, aiResponse.Title, nil
}

// ... internal mock functions remain the same but capitalized or private as per package needs ...
func generateMockWorkouts(_ float64, days int, availability map[string]int, userName string) ([]models.DayWorkout, string) {
	workouts := make([]models.DayWorkout, days)
	title := "Desafío de Resistencia"

	types := []struct {
		name      string
		wType     string
		duration  int
		intervals []models.Interval
	}{
		{"Base Aeróbica", "endurance", 60, []models.Interval{
			{Type: "steady", Duration: 600, Power: 0.5},
			{Type: "steady", Duration: 2400, Power: 0.7},
			{Type: "steady", Duration: 600, Power: 0.5},
		}},
		{"Intervalos de Umbral", "threshold", 60, []models.Interval{
			{Type: "steady", Duration: 900, Power: 0.5},
			{Type: "interval", Repeat: 4, OnDuration: 480, OffDuration: 240, OnPower: 0.95, OffPower: 0.5},
			{Type: "steady", Duration: 600, Power: 0.4},
		}},
		{"Series VO2max", "vo2max", 45, []models.Interval{
			{Type: "steady", Duration: 600, Power: 0.5},
			{Type: "interval", Repeat: 5, OnDuration: 180, OffDuration: 180, OnPower: 1.15, OffPower: 0.4},
			{Type: "steady", Duration: 600, Power: 0.4},
		}},
	}

	startDate := time.Now()
	weekdays := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

	for i := 0; i < days; i++ {
		currentDate := startDate.AddDate(0, 0, i)
		dayOfWeek := weekdays[currentDate.Weekday()]
		maxMinutes := availability[dayOfWeek]
		if maxMinutes == 0 && availability != nil {
			workouts[i] = models.DayWorkout{Day: i + 1, Name: fmt.Sprintf("%s - Descanso", userName), Type: "rest", Duration: 0}
			continue
		}
		if maxMinutes == 0 {
			maxMinutes = 60
		}

		t := types[i%len(types)]
		duration := t.duration
		if duration > maxMinutes {
			duration = maxMinutes
		}

		workouts[i] = models.DayWorkout{
			Day:       i + 1,
			Name:      fmt.Sprintf("%s - %s", userName, t.name),
			Type:      t.wType,
			Duration:  duration,
			Intervals: t.intervals,
		}
	}
	return workouts, title
}
