package jobs

import (
	"cmp"
	"slices"
	"strings"
)

// ProgressReport is the shareable part of a single application. It stays on
// the server. PositionProgress returns the combined counts.
type ProgressReport struct {
	Status string
	Stages []string
}

type PositionProgress struct {
	Total       int            `json:"total"`
	Stages      []ProgressNode `json:"stages"`
	Transitions []ProgressLink `json:"transitions"`
	Suggestions []string       `json:"suggestions"`
}

type ProgressNode struct {
	ID      int    `json:"id"`
	Round   int    `json:"round"`
	Title   string `json:"title"`
	Reached int    `json:"reached"`
	Current int    `json:"current"`
}

type ProgressLink struct {
	Source int `json:"source"`
	Target int `json:"target"`
	Count  int `json:"count"`
}

func SummarizeProgress(reports []ProgressReport) PositionProgress {
	result := PositionProgress{
		Total: len(reports), Stages: []ProgressNode{}, Transitions: []ProgressLink{},
		Suggestions: []string{},
	}
	type nodeKey struct {
		round int
		name  string
	}
	nodes := map[nodeKey]int{}
	links := map[[2]int]int{}
	titles := map[string]string{}
	frequency := map[string]int{}
	for _, report := range reports {
		reached := []string{"Applied"}
		seen := map[string]bool{}
		for _, stage := range report.Stages {
			title := strings.Join(strings.Fields(stage), " ")
			key := strings.ToLower(title)
			switch key {
			case "", "applied", "applied (merged listing)", "active", "rejected", "offer":
				continue
			}
			if _, exists := titles[key]; !exists {
				titles[key] = title
			}
			if !seen[key] {
				frequency[key]++
				seen[key] = true
			}
			reached = append(reached, titles[key])
		}
		switch report.Status {
		case "inactive":
			reached = append(reached, "Rejected")
		case "offer":
			reached = append(reached, "Offer")
		}
		var previous int
		for round, title := range reached {
			key := nodeKey{round, strings.ToLower(title)}
			id, exists := nodes[key]
			if !exists {
				id = len(result.Stages)
				nodes[key] = id
				result.Stages = append(result.Stages, ProgressNode{ID: id, Round: round, Title: title})
			}
			result.Stages[id].Reached++
			if round == len(reached)-1 {
				result.Stages[id].Current++
			}
			if round > 0 {
				pair := [2]int{previous, id}
				index, exists := links[pair]
				if !exists {
					index = len(result.Transitions)
					links[pair] = index
					result.Transitions = append(result.Transitions, ProgressLink{Source: previous, Target: id})
				}
				result.Transitions[index].Count++
			}
			previous = id
		}
	}
	for _, key := range rankedStageNames(frequency) {
		result.Suggestions = append(result.Suggestions, titles[key])
	}
	return result
}

func rankedStageNames(counts map[string]int) []string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b string) int {
		if byCount := cmp.Compare(counts[b], counts[a]); byCount != 0 {
			return byCount
		}
		return cmp.Compare(a, b)
	})
	return keys
}
