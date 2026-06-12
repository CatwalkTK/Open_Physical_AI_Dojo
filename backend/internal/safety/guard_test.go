package safety

import (
	"testing"

	"open-physical-ai-dojo/backend/internal/domain"
)

func validPlan() domain.ActionPlan {
	return domain.ActionPlan{
		Goal:      "approach_target",
		RiskLevel: "low",
		Steps: []domain.ActionStep{
			{Type: "stand"},
			{Type: "move", LinearX: 0.08, DurationMS: 1200},
			{Type: "stop"},
		},
	}
}

func TestValidatePlanSimulatorBypassesLimits(t *testing.T) {
	plan := domain.ActionPlan{
		RiskLevel: "high",
		Steps: []domain.ActionStep{
			{Type: "move", LinearX: 99, YawDeg: 999, DurationMS: 999999},
		},
	}
	if err := NewGuard().ValidatePlan(domain.EnvironmentSimulator, plan); err != nil {
		t.Fatalf("simulator plans must not be restricted, got: %v", err)
	}
}

func TestValidatePlanDogzillaAcceptsSafePlan(t *testing.T) {
	if err := NewGuard().ValidatePlan(domain.EnvironmentDogzilla, validPlan()); err != nil {
		t.Fatalf("expected safe plan to pass, got: %v", err)
	}
}

func TestValidatePlanDogzillaRejections(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*domain.ActionPlan)
	}{
		{"high risk", func(p *domain.ActionPlan) { p.RiskLevel = "high" }},
		{"empty steps", func(p *domain.ActionPlan) { p.Steps = nil }},
		{"unsupported step", func(p *domain.ActionPlan) {
			p.Steps = append(p.Steps, domain.ActionStep{Type: "jump"})
		}},
		{"linear_x too fast", func(p *domain.ActionPlan) {
			p.Steps[1].LinearX = MaxLinearX + 0.01
		}},
		{"linear_x too fast backwards", func(p *domain.ActionPlan) {
			p.Steps[1].LinearX = MinLinearX - 0.01
		}},
		{"linear_y too fast", func(p *domain.ActionPlan) {
			p.Steps[1].LinearY = MaxLinearY + 0.01
		}},
		{"yaw too large", func(p *domain.ActionPlan) {
			p.Steps[1].YawDeg = MaxYawDeg + 1
		}},
		{"step duration too long", func(p *domain.ActionPlan) {
			p.Steps[1].DurationMS = MaxStepDurationMS + 1
		}},
		{"total duration too long", func(p *domain.ActionPlan) {
			p.Steps = []domain.ActionStep{
				{Type: "move", LinearX: 0.05, DurationMS: 1800},
				{Type: "move", LinearX: 0.05, DurationMS: 1800},
				{Type: "move", LinearX: 0.05, DurationMS: 1800},
				{Type: "move", LinearX: 0.05, DurationMS: 1800},
				{Type: "stop"},
			}
		}},
		{"total distance too long", func(p *domain.ActionPlan) {
			p.Steps = []domain.ActionStep{
				{Type: "move", LinearX: 0.12, DurationMS: 1800},
				{Type: "move", LinearX: 0.12, DurationMS: 1800},
				{Type: "move", LinearX: 0.12, DurationMS: 1800},
				{Type: "stop"},
			}
		}},
	}

	guard := NewGuard()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := validPlan()
			tt.mutate(&plan)
			if err := guard.ValidatePlan(domain.EnvironmentDogzilla, plan); err == nil {
				t.Fatal("expected plan to be rejected")
			}
		})
	}
}

func TestValidatePlanCountsZeroDurationAsMinimum(t *testing.T) {
	// 21 zero-duration moves are treated as 21 * 300ms = 6300ms > limit.
	steps := make([]domain.ActionStep, 0, 22)
	for i := 0; i < 21; i++ {
		steps = append(steps, domain.ActionStep{Type: "move", LinearX: 0.01})
	}
	steps = append(steps, domain.ActionStep{Type: "stop"})
	plan := domain.ActionPlan{RiskLevel: "low", Steps: steps}

	if err := NewGuard().ValidatePlan(domain.EnvironmentDogzilla, plan); err == nil {
		t.Fatal("expected zero-duration moves to count against total duration")
	}
}
