package safety

import (
	"errors"

	"open-physical-ai-dojo/backend/internal/domain"
)

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
	for _, step := range plan.Steps {
		if step.Type == "move" {
			if step.LinearX > 0.12 || step.LinearX < -0.08 {
				return errors.New("Dogzilla linear_x exceeds safety limit")
			}
			if step.LinearY > 0.08 || step.LinearY < -0.08 {
				return errors.New("Dogzilla linear_y exceeds safety limit")
			}
			if step.YawDeg > 30 || step.YawDeg < -30 {
				return errors.New("Dogzilla yaw exceeds safety limit")
			}
			if step.DurationMS > 1800 {
				return errors.New("Dogzilla move duration exceeds safety limit")
			}
		}
	}
	return nil
}
