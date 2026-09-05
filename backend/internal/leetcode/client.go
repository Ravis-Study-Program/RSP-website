// Package leetcode synchronizes the LeetCode problem catalog.
package leetcode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/worker"
)

type Problem struct {
	Number     int      `json:"number"`
	Title      string   `json:"title"`
	Slug       string   `json:"slug"`
	Difficulty string   `json:"difficulty"`
	Premium    bool     `json:"premium"`
	Categories []string `json:"categories"`
}

type Sink interface {
	Upsert(context.Context, Problem) error
}

type Client struct {
	URL  string
	HTTP *http.Client
	Sink Sink
}

const DefaultURL = "https://leetcode.com/graphql/"

const catalogQuery = `query problemsetQuestionList($categorySlug: String, $limit: Int, $skip: Int, $filters: QuestionListFilterInput) {
  problemsetQuestionList: questionList(categorySlug: $categorySlug, limit: $limit, skip: $skip, filters: $filters) {
    questions: data { difficulty premium: isPaidOnly questionId: questionFrontendId title titleSlug topicTags { name } }
  }
}`

// Sync synchronizes data.
func (c Client) Sync(ctx context.Context) (worker.Report, error) {
	if c.Sink == nil {
		return worker.Report{}, errors.New("LeetCode sink is required")
	}
	endpoint := c.URL
	if endpoint == "" {
		endpoint = DefaultURL
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	body, err := json.Marshal(map[string]any{
		"query":     catalogQuery,
		"variables": map[string]any{"categorySlug": "", "skip": 0, "limit": 10000, "filters": map[string]any{}},
	})
	if err != nil {
		return worker.Report{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return worker.Report{}, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Referer", "https://leetcode.com/problemset/")
	res, err := client.Do(req)
	if err != nil {
		return worker.Report{}, err
	}

	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return worker.Report{}, fmt.Errorf("leetcode catalog returned %d", res.StatusCode)
	}
	var payload struct {
		Data struct {
			Problemset struct {
				Questions []struct {
					Difficulty string `json:"difficulty"`
					Premium    bool   `json:"premium"`
					QuestionID string `json:"questionId"`
					Title      string `json:"title"`
					TitleSlug  string `json:"titleSlug"`
					TopicTags  []struct {
						Name string `json:"name"`
					} `json:"topicTags"`
				} `json:"questions"`
			} `json:"problemsetQuestionList"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	dec := json.NewDecoder(http.MaxBytesReader(nil, res.Body, 8<<20))
	if err := dec.Decode(&payload); err != nil {
		return worker.Report{}, err
	}
	if len(payload.Errors) > 0 {
		return worker.Report{}, fmt.Errorf("leetcode GraphQL error: %s", payload.Errors[0].Message)
	}

	problems := make([]Problem, 0, len(payload.Data.Problemset.Questions))
	for _, raw := range payload.Data.Problemset.Questions {
		number, numberErr := strconv.Atoi(raw.QuestionID)
		categories := make([]string, 0, len(raw.TopicTags))
		for _, tag := range raw.TopicTags {
			if name := strings.TrimSpace(tag.Name); name != "" {
				categories = append(categories, name)
			}
		}
		p := Problem{Number: number, Title: strings.TrimSpace(raw.Title), Slug: strings.TrimSpace(raw.TitleSlug), Difficulty: strings.ToLower(raw.Difficulty), Premium: raw.Premium, Categories: categories}
		if numberErr != nil {
			p.Number = 0
		}
		problems = append(problems, p)
	}

	report := worker.Report{Fetched: len(problems)}
	for _, p := range problems {
		if p.Number <= 0 || p.Title == "" || (p.Difficulty != "easy" && p.Difficulty != "medium" && p.Difficulty != "hard") {
			report.Failed++
			continue
		}
		if err := c.Sink.Upsert(ctx, p); err != nil {
			report.Failed++
			continue
		}

		report.Updated++
	}

	return report, nil
}
