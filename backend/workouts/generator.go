package workouts

import (
	"fmt"
	"strings"

	"cycling-trainer-agent-ai/backend/models"
)

// GenerateZWOFromWorkout converts a structured DayWorkout into a valid ZWO XML file
func GenerateZWOFromWorkout(workout models.DayWorkout, ftp float64) string {
	var blocks strings.Builder

	for _, interval := range workout.Intervals {
		switch interval.Type {
		case "warmup":
			// Warmup ramps from low to target power
			startPower := interval.Power * 0.5
			if startPower < 0.25 {
				startPower = 0.25
			}
			blocks.WriteString(fmt.Sprintf(
				"        <Warmup Duration=\"%d\" PowerLow=\"%.2f\" PowerHigh=\"%.2f\"/>\n",
				interval.Duration, startPower, interval.Power))

		case "cooldown":
			// Cooldown ramps from target power down
			endPower := interval.Power * 0.5
			if endPower < 0.25 {
				endPower = 0.25
			}
			blocks.WriteString(fmt.Sprintf(
				"        <Cooldown Duration=\"%d\" PowerLow=\"%.2f\" PowerHigh=\"%.2f\"/>\n",
				interval.Duration, interval.Power, endPower))

		case "interval":
			// Structured intervals with work/rest
			blocks.WriteString(fmt.Sprintf(
				"        <IntervalsT Repeat=\"%d\" OnDuration=\"%d\" OffDuration=\"%d\" OnPower=\"%.2f\" OffPower=\"%.2f\"/>\n",
				interval.Repeat, interval.OnDuration, interval.OffDuration,
				interval.OnPower, interval.OffPower))

		default: // "steady" or any other
			blocks.WriteString(fmt.Sprintf(
				"        <SteadyState Duration=\"%d\" Power=\"%.2f\"/>\n",
				interval.Duration, interval.Power))
		}
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<workout_file>
    <author>AI Cycling Coach</author>
    <name>Día %d - %s</name>
    <description>%s - Duración: %d min - Generado por CyclingAI</description>
    <sportType>bike</sportType>
    <tags>
        <tag name="%s"/>
    </tags>
    <workout>
%s    </workout>
</workout_file>`, workout.Day, workout.Name, workout.Name, workout.Duration, workout.Type, blocks.String())
}
