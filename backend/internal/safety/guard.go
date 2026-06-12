package safety

import (
	"errors"
	"fmt"

	"open-physical-ai-dojo/backend/internal/domain"
)

// Safety limits for real-robot (Dogzilla) execution. These are the single
// source of truth; other packages should reference them instead of
// hardcoding values.
const (
	MaxLinearX = 0.12
	MinLinearX = -0.08
	MaxLinearY = 0.08
	MinLinearY = -0.08
	MaxYawDeg  = 30.0

	// Per-step and per-plan execution time limits.
	MaxStepDurationMS  = 1800
	MaxTotalDurationMS = 6000

	// Maximum cumulative travel distance for one plan, in meters.
	MaxTotalDistanceM = 0.5

	// Executors treat shorter/zero durations as this minimum, so the guard
	// must account for it when estimating time and distance.
	MinEffectiveDurationMS = 300
)

var allowedDogzillaSteps = map[string]bool{
	"stand": true,
	"sit":   true,
	"move":  true,
	"stop":  true,
}

type Guard struct{}

func NewGuard() Guard {
	return Guard{}
}

func (g Guard) ValidatePlan(environment domain.Environment, plan domain.ActionPlan) error {
	if environment != domain.EnvironmentDogzilla {
		return nil
	}
	if plan.RiskLevel == "high" {
		return errors.New("high risk plans cannot run on Dogzilla")
	}
	if len(plan.Steps) == 0 {
		return errors.New("plan has no steps")
	}

	totalDurationMS := 0
	totalDistanceM := 0.0
	for _, step := range plan.Steps {
		if !allowedDogzillaSteps[step.Type] {
			return fmt.Errorf("step type %q is not allowed on Dogzilla", step.Type)
		}
		if step.Type != "move" {
			continue
		}
		if step.LinearX > MaxLinearX || step.LinearX < MinLinearX {
			return errors.New("Dogzilla linear_x exceeds safety limit")
		}
		if step.LinearY > MaxLinearY || step.LinearY < MinLinearY {
			return errors.New("Dogzilla linear_y exceeds safety limit")
		}
		if step.YawDeg > MaxYawDeg || step.YawDeg < -MaxYawDeg {
			return errors.New("Dogzilla yaw exceeds safety limit")
		}
		if step.DurationMS > MaxStepDurationMS {
			return errors.New("Dogzilla move duration exceeds safety limit")
		}

		effectiveMS := step.DurationMS
		if effectiveMS < MinEffectiveDurationMS {
			effectiveMS = MinEffectiveDurationMS
		}
		totalDurationMS += effectiveMS
		totalDistanceM += abs(step.LinearX) * float64(effectiveMS) / 1000
	}

	if totalDurationMS > MaxTotalDurationMS {
		return errors.New("Dogzilla plan total move duration exceeds safety limit")
	}
	if totalDistanceM > MaxTotalDistanceM {
		return errors.New("Dogzilla plan total travel distance exceeds safety limit")
	}
	return nil
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
