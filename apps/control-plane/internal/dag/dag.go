package dag

import (
	"fmt"
	"opspilot/control-plane/internal/models"
)

const (
	StepBlockedByDependency = "BLOCKED_BY_DEPENDENCY"
)

// Node represents a vertex in the DAG
type Node struct {
	ID        string
	DependsOn []string
	Step      *models.TaskStep
}

// Graph represents the Directed Acyclic Graph of execution steps
type Graph struct {
	Nodes map[string]*Node
}

func NewGraph() *Graph {
	return &Graph{
		Nodes: make(map[string]*Node),
	}
}

func (g *Graph) AddNode(step *models.TaskStep) {
	deps := step.DependsOn
	if deps == nil {
		deps = []string{}
	}
	g.Nodes[step.ID] = &Node{
		ID:        step.ID,
		DependsOn: deps,
		Step:      step,
	}
}

// Validate checks for cycle detection using Kahn's algorithm and verifies that all dependencies exist
func (g *Graph) Validate() error {
	inDegree := make(map[string]int)
	adjacency := make(map[string][]string)

	for id := range g.Nodes {
		inDegree[id] = 0
	}

	for id, node := range g.Nodes {
		for _, dep := range node.DependsOn {
			if _, exists := g.Nodes[dep]; !exists {
				return fmt.Errorf("node %s depends on non-existent step %s", id, dep)
			}
			adjacency[dep] = append(adjacency[dep], id)
			inDegree[id]++
		}
	}

	// Find nodes with zero in-degree
	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	visitedCount := 0
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		visitedCount++

		for _, neighbor := range adjacency[curr] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if visitedCount != len(g.Nodes) {
		return fmt.Errorf("cycle detected in DAG: %d of %d nodes visited", visitedCount, len(g.Nodes))
	}

	return nil
}

// ExecutionBatches returns the levels of the DAG where each level can be executed concurrently
func (g *Graph) ExecutionBatches() ([][]*models.TaskStep, error) {
	if err := g.Validate(); err != nil {
		return nil, err
	}

	inDegree := make(map[string]int)
	adjacency := make(map[string][]string)

	for id := range g.Nodes {
		inDegree[id] = 0
	}

	for id, node := range g.Nodes {
		for _, dep := range node.DependsOn {
			adjacency[dep] = append(adjacency[dep], id)
			inDegree[id]++
		}
	}

	var currentBatch []string
	for id, deg := range inDegree {
		if deg == 0 {
			currentBatch = append(currentBatch, id)
		}
	}

	var batches [][]*models.TaskStep

	for len(currentBatch) > 0 {
		var stepBatch []*models.TaskStep
		var nextBatch []string

		for _, id := range currentBatch {
			stepBatch = append(stepBatch, g.Nodes[id].Step)

			for _, neighbor := range adjacency[id] {
				inDegree[neighbor]--
				if inDegree[neighbor] == 0 {
					nextBatch = append(nextBatch, neighbor)
				}
			}
		}

		batches = append(batches, stepBatch)
		currentBatch = nextBatch
	}

	return batches, nil
}

// ReadyNodes returns nodes whose dependencies are all satisfied in completedNodes
func (g *Graph) ReadyNodes(completedNodes map[string]bool, inFlightNodes map[string]bool) []*models.TaskStep {
	var ready []*models.TaskStep

	for id, node := range g.Nodes {
		if completedNodes[id] || inFlightNodes[id] {
			continue
		}

		allDepsSatisfied := true
		for _, dep := range node.DependsOn {
			if !completedNodes[dep] {
				allDepsSatisfied = false
				break
			}
		}

		if allDepsSatisfied {
			ready = append(ready, node.Step)
		}
	}

	return ready
}

// PropagateFailure identifies all downstream dependents of a failed step and marks them BLOCKED_BY_DEPENDENCY
func (g *Graph) PropagateFailure(failedStepID string) []string {
	adjacency := make(map[string][]string)
	for id, node := range g.Nodes {
		for _, dep := range node.DependsOn {
			adjacency[dep] = append(adjacency[dep], id)
		}
	}

	var blocked []string
	visited := make(map[string]bool)
	queue := []string{failedStepID}
	visited[failedStepID] = true

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		for _, dependent := range adjacency[curr] {
			if !visited[dependent] {
				visited[dependent] = true
				blocked = append(blocked, dependent)
				queue = append(queue, dependent)
				if g.Nodes[dependent] != nil && g.Nodes[dependent].Step != nil {
					g.Nodes[dependent].Step.Status = StepBlockedByDependency
				}
			}
		}
	}

	return blocked
}
