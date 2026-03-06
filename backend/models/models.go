package models

import "github.com/golang-jwt/jwt/v5"

type User struct {
	Email     string  `json:"email" dynamodbav:"PK"` // USER#email
	SK        string  `json:"-" dynamodbav:"SK"`     // PROFILE
	Password  string  `json:"password,omitempty" dynamodbav:"password"`
	FirstName string  `json:"first_name" dynamodbav:"first_name"`
	FTP       float64 `json:"ftp" dynamodbav:"ftp"`
	Weight    float64 `json:"weight" dynamodbav:"weight"`
}

type FTPRecord struct {
	PK        string  `json:"-" dynamodbav:"PK"` // USER#email
	SK        string  `json:"-" dynamodbav:"SK"` // FTP#timestamp
	FTP       float64 `json:"ftp" dynamodbav:"ftp"`
	Weight    float64 `json:"weight" dynamodbav:"weight"`
	Timestamp string  `json:"timestamp" dynamodbav:"timestamp"`
}

// Interval represents a single block in a workout (e.g., warmup, steady state, intervals)
type Interval struct {
	Type     string  `json:"type"`     // "warmup", "cooldown", "steady", "interval"
	Duration int     `json:"duration"` // seconds
	Power    float64 `json:"power"`    // percentage of FTP (0.0 - 1.5+)
	// For interval type only
	OnDuration  int     `json:"on_duration,omitempty"`  // work seconds
	OffDuration int     `json:"off_duration,omitempty"` // rest seconds
	OnPower     float64 `json:"on_power,omitempty"`     // work power % FTP
	OffPower    float64 `json:"off_power,omitempty"`    // rest power % FTP
	Repeat      int     `json:"repeat,omitempty"`       // number of repetitions
}

// DayWorkout is the structured response from the AI for a single day
type DayWorkout struct {
	Day       int        `json:"day"`
	Name      string     `json:"name"`
	Title     string     `json:"title"`
	Type      string     `json:"type"` // "endurance", "intervals", "rest", "vo2max", "threshold", "recovery"
	Duration  int        `json:"duration_minutes"`
	Intervals []Interval `json:"intervals"`
}

// WorkoutDay is stored in DynamoDB per day
type WorkoutDay struct {
	PK        string     `json:"pk" dynamodbav:"PK"` // USER#email
	SK        string     `json:"sk" dynamodbav:"SK"` // DAY#N
	PlanID    string     `json:"plan_id" dynamodbav:"plan_id"`
	IsActive  bool       `json:"is_active" dynamodbav:"is_active"`
	Day       int        `json:"day" dynamodbav:"day"`
	Name      string     `json:"name" dynamodbav:"name"`
	Title     string     `json:"title" dynamodbav:"title"`
	Type      string     `json:"type" dynamodbav:"type"`
	Duration  int        `json:"duration_minutes" dynamodbav:"duration_minutes"`
	Intervals []Interval `json:"intervals" dynamodbav:"intervals"`
	ZWO       string     `json:"zwo" dynamodbav:"zwo"`
	Goal      string     `json:"goal" dynamodbav:"goal"`
	Completed bool       `json:"completed" dynamodbav:"completed"`
	CreatedAt string     `json:"created_at" dynamodbav:"created_at"`
}

type Claims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}
