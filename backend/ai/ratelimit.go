package ai

import (
	"fmt"
	"sync"
	"time"
)

const (
	MaxRequestsPerDay = 10  // Máximo de peticiones por usuario por día
	MaxGoalLength     = 500 // Máximo de caracteres en el objetivo
)

type userUsage struct {
	Count     int
	ResetTime time.Time
}

type RateLimiter struct {
	mu    sync.Mutex
	users map[string]*userUsage
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		users: make(map[string]*userUsage),
	}
}

// CheckLimit verifica si el usuario puede hacer una petición.
// Retorna nil si está permitido, o un error descriptivo si excede el límite.
func (rl *RateLimiter) CheckLimit(email string) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	usage, exists := rl.users[email]

	if !exists || now.After(usage.ResetTime) {
		// Primer uso o se reinició el día
		rl.users[email] = &userUsage{
			Count:     1,
			ResetTime: now.Add(24 * time.Hour),
		}
		fmt.Printf(">> Rate Limit: %s -> 1/%d peticiones usadas (reinicio: %s)\n",
			email, MaxRequestsPerDay, rl.users[email].ResetTime.Format("15:04"))
		return nil
	}

	if usage.Count >= MaxRequestsPerDay {
		remaining := time.Until(usage.ResetTime).Round(time.Minute)
		fmt.Printf("!! Rate Limit: %s ha excedido el límite (%d/%d). Reinicio en %v\n",
			email, usage.Count, MaxRequestsPerDay, remaining)
		return fmt.Errorf("Has alcanzado el límite de %d planes por día. Intenta de nuevo en %v",
			MaxRequestsPerDay, remaining)
	}

	usage.Count++
	fmt.Printf(">> Rate Limit: %s -> %d/%d peticiones usadas\n",
		email, usage.Count, MaxRequestsPerDay)
	return nil
}

// GetUsage retorna el uso actual de un usuario
func (rl *RateLimiter) GetUsage(email string) (count int, max int, resetIn time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	usage, exists := rl.users[email]
	if !exists || time.Now().After(usage.ResetTime) {
		return 0, MaxRequestsPerDay, 0
	}
	return usage.Count, MaxRequestsPerDay, time.Until(usage.ResetTime)
}

// ValidateGoalInput valida la longitud del texto del objetivo
func ValidateGoalInput(goal string) error {
	if goal == "" {
		return fmt.Errorf("El objetivo no puede estar vacío")
	}
	if len(goal) > MaxGoalLength {
		return fmt.Errorf("El objetivo es demasiado largo (%d caracteres). Máximo permitido: %d",
			len(goal), MaxGoalLength)
	}
	return nil
}
