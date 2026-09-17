package dag

import (
	"testing"

	"opspilot/control-plane/internal/models"
)

func TestDAGValidationAndBatches(t *testing.T) {
	// A and B have 0 deps (can run parallel)
	// C depends on A and B
	// D depends on C
	g := NewGraph()

	stepA := &models.TaskStep{ID: "step-A", Action: "install_package", DependsOn: []string{}}
	stepB := &models.TaskStep{ID: "step-B", Action: "write_file", DependsOn: []string{}}
	stepC := &models.TaskStep{ID: "step-C", Action: "restart_service", DependsOn: []string{"step-A", "step-B"}}
	stepD := &models.TaskStep{ID: "step-D", Action: "http_probe", DependsOn: []string{"step-C"}}

	g.AddNode(stepA)
	g.AddNode(stepB)
	g.AddNode(stepC)
	g.AddNode(stepD)

	if err := g.Validate(); err != nil {
		t.Fatalf("expected valid DAG, got: %v", err)
	}

	batches, err := g.ExecutionBatches()
	if err != nil {
		t.Fatalf("failed to calculate batches: %v", err)
	}

	if len(batches) != 3 {
		t.Fatalf("expected 3 execution levels, got %d", len(batches))
	}

	// Level 0 should have A and B
	if len(batches[0]) != 2 {
		t.Errorf("expected 2 steps in level 0, got %d", len(batches[0]))
	}

	// Level 1 should have C
	if len(batches[1]) != 1 || batches[1][0].ID != "step-C" {
		t.Errorf("expected step-C in level 1, got %+v", batches[1])
	}

	// Level 2 should have D
	if len(batches[2]) != 1 || batches[2][0].ID != "step-D" {
		t.Errorf("expected step-D in level 2, got %+v", batches[2])
	}
}

func TestDAGCycleDetection(t *testing.T) {
	// A -> B -> C -> A (Cycle!)
	g := NewGraph()
	g.AddNode(&models.TaskStep{ID: "step-A", DependsOn: []string{"step-C"}})
	g.AddNode(&models.TaskStep{ID: "step-B", DependsOn: []string{"step-A"}})
	g.AddNode(&models.TaskStep{ID: "step-C", DependsOn: []string{"step-B"}})

	err := g.Validate()
	if err == nil {
		t.Fatalf("expected cycle detection error, got nil")
	}
}

func TestDAGFailurePropagation(t *testing.T) {
	// A -> C -> D
	// B -> E
	g := NewGraph()
	stepA := &models.TaskStep{ID: "step-A", DependsOn: []string{}}
	stepB := &models.TaskStep{ID: "step-B", DependsOn: []string{}}
	stepC := &models.TaskStep{ID: "step-C", DependsOn: []string{"step-A"}}
	stepD := &models.TaskStep{ID: "step-D", DependsOn: []string{"step-C"}}
	stepE := &models.TaskStep{ID: "step-E", DependsOn: []string{"step-B"}}

	g.AddNode(stepA)
	g.AddNode(stepB)
	g.AddNode(stepC)
	g.AddNode(stepD)
	g.AddNode(stepE)

	// Failure in step-A should block C and D, but NOT B or E!
	blocked := g.PropagateFailure("step-A")
	if len(blocked) != 2 {
		t.Fatalf("expected 2 blocked nodes (C and D), got %d: %v", len(blocked), blocked)
	}

	if stepC.Status != StepBlockedByDependency {
		t.Errorf("expected step-C to be BLOCKED_BY_DEPENDENCY, got %s", stepC.Status)
	}
	if stepD.Status != StepBlockedByDependency {
		t.Errorf("expected step-D to be BLOCKED_BY_DEPENDENCY, got %s", stepD.Status)
	}
	if stepE.Status == StepBlockedByDependency {
		t.Errorf("step-E should not be blocked by step-A failure")
	}
}
