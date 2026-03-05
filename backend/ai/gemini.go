package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"cycling-trainer-agent-ai/backend/models"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GenerateWorkouts asks Gemini for structured workout data and returns parsed DayWorkout slices and a concise title
func GenerateWorkouts(ctx context.Context, goal string, ftp float64, weight float64, days int, availability map[string]int) ([]models.DayWorkout, string, error) {
	// Modo Mock para desarrollo local sin consumir cuota
	if os.Getenv("MOCK_AI") == "true" {
		fmt.Println(">> [MOCK] Generando entrenamientos simulados...")
		workouts, title := generateMockWorkouts(ftp, days, availability)
		return workouts, title, nil
	}

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Println("!! GEMINI_API_KEY no está configurada")
		return nil, "", fmt.Errorf("GEMINI_API_KEY is not set")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		fmt.Printf("!! Error creando cliente GenAI: %v\n", err)
		return nil, "", err
	}
	defer client.Close()

	// gemini-2.5-flash: Mejor balance calidad/costo para planes de entrenamiento
	modelName := "gemini-2.5-flash"
	model := client.GenerativeModel(modelName)
	model.ResponseMIMEType = "application/json"

	availStr, _ := json.Marshal(availability)

	prompt := fmt.Sprintf(
		"Actúa como un entrenador de ciclismo experto. Genera un plan de entrenamiento de exactamente %d días para un ciclista con un objetivo de: '%s'. "+
			"Disponibilidad semanal (minutos por día): %s. "+
			"Reglas CRÍTICAS: "+
			"1. Genera un TÍTULO corto y motivador (máximo 5 palabras) para este objetivo. "+
			"2. Respeta ESTRICTAMENTE la disponibilidad de tiempo. La duración total de cada día (duration_minutes) NO PUEDE superar los minutos disponibles para ese día de la semana. "+
			"3. Si la disponibilidad es 0 o muy baja (ej: < 30min), el día debe ser 'rest'. "+
			"4. Es CRÍTICO que el resultado sea un objeto JSON con este formato exacto: "+
			"{ \"title\": \"Título corto\", \"workouts\": [ { \"day\": 1, \"name\": \"...\", \"type\": \"...\", \"duration_minutes\": ..., \"intervals\": [...] }, ... ] }. "+
			"Genera EXACTAMENTE %d días en el array 'workouts', del 1 al %d consecutivo. No agrupes días ni omitas ninguno. "+
			"El FTP del usuario es %.0f W y su peso es %.1f kg. Regresa SOLAMENTE el JSON, sin bloques de código.",
		days, goal, string(availStr), days, days, ftp, weight,
	)

	fmt.Printf(">> Enviando petición a Gemini (Modelo: %s, Días: %d)...\n", modelName, days)
	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		fmt.Printf("!! Error llamando a Gemini (%s): %v\n", modelName, err)
		return nil, "", err
	}

	if len(resp.Candidates) == 0 {
		return nil, "", fmt.Errorf("sin respuesta de Gemini")
	}

	responseText := ""
	for _, part := range resp.Candidates[0].Content.Parts {
		responseText += fmt.Sprintf("%v", part)
	}

	// Limpiar posible markdown
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimSuffix(responseText, "```")
	responseText = strings.TrimSpace(responseText)

	var aiResponse struct {
		Title    string              `json:"title"`
		Workouts []models.DayWorkout `json:"workouts"`
	}
	err = json.Unmarshal([]byte(responseText), &aiResponse)
	if err != nil {
		fmt.Printf("!! Error decodificando JSON de IA: %v\n", err)
		fmt.Printf(">> Respuesta raw: %s\n", responseText)
		return nil, "", fmt.Errorf("error parseando respuesta de IA: %v", err)
	}

	return aiResponse.Workouts, aiResponse.Title, nil
}

func generateMockWorkouts(_ float64, days int, availability map[string]int) ([]models.DayWorkout, string) {
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
			// Si el mapa existe pero es 0, es descanso
			workouts[i] = models.DayWorkout{Day: i + 1, Name: "Descanso", Type: "rest", Duration: 0}
			continue
		}
		if maxMinutes == 0 {
			maxMinutes = 60
		} // Fallback

		t := types[i%len(types)]
		duration := t.duration
		if duration > maxMinutes {
			duration = maxMinutes
		}

		workouts[i] = models.DayWorkout{
			Day:       i + 1,
			Name:      t.name,
			Type:      t.wType,
			Duration:  duration,
			Intervals: t.intervals,
		}
	}
	return workouts, title
}
