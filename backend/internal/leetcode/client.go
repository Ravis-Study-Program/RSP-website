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
	UpsertProblem(context.Context, Problem) error
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

// SyncCatalogue fetches the catalogue and records how many problems were applied.
func (c Client) SyncCatalogue(ctx context.Context) (worker.Report, error) {
	if c.Sink == nil {
		return worker.Report{}, errors.New("LeetCode sink is required")
	}
	questions, err := c.fetchCatalogueQuestions(ctx)
	if err != nil {
		return worker.Report{}, err
	}
	problems := make([]Problem, 0, len(questions))
	for _, question := range questions {
		problems = append(problems, normalizeQuestion(question))
	}

	report := worker.Report{Fetched: len(problems)}
	for _, p := range problems {
		if p.Number <= 0 || p.Title == "" || (p.Difficulty != "easy" && p.Difficulty != "medium" && p.Difficulty != "hard") {
			report.Failed++
			continue
		}
		if err := c.Sink.UpsertProblem(ctx, p); err != nil {
			report.Failed++
			continue
		}

		report.Applied++
	}

	return report, nil
}

type catalogueQuestion struct {
	Difficulty string `json:"difficulty"`
	Premium    bool   `json:"premium"`
	QuestionID string `json:"questionId"`
	Title      string `json:"title"`
	TitleSlug  string `json:"titleSlug"`
	TopicTags  []struct {
		Name string `json:"name"`
	} `json:"topicTags"`
}

type catalogueResponse struct {
	Data struct {
		Problemset struct {
			Questions []catalogueQuestion `json:"questions"`
		} `json:"problemsetQuestionList"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (c Client) fetchCatalogueQuestions(ctx context.Context) ([]catalogueQuestion, error) {
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
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Referer", "https://leetcode.com/problemset/")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("leetcode catalog returned %d", res.StatusCode)
	}
	var payload catalogueResponse

	dec := json.NewDecoder(http.MaxBytesReader(nil, res.Body, 8<<20))
	if err := dec.Decode(&payload); err != nil {
		return nil, err
	}
	if len(payload.Errors) > 0 {
		return nil, fmt.Errorf("leetcode GraphQL error: %s", payload.Errors[0].Message)
	}

	return payload.Data.Problemset.Questions, nil
}

func normalizeQuestion(question catalogueQuestion) Problem {
	number, err := strconv.Atoi(question.QuestionID)
	if err != nil {
		number = 0
	}
	categories := make([]string, 0, len(question.TopicTags))
	for _, tag := range question.TopicTags {
		if name := strings.TrimSpace(tag.Name); name != "" {
			categories = append(categories, name)
		}
	}
	return Problem{
		Number:     number,
		Title:      strings.TrimSpace(question.Title),
		Slug:       strings.TrimSpace(question.TitleSlug),
		Difficulty: strings.ToLower(question.Difficulty),
		Premium:    question.Premium,
		Categories: categories,
	}
}
