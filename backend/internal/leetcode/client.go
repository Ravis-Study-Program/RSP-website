package leetcode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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

func (c Client) Sync(ctx context.Context) (worker.Report, error) {
	if c.URL == "" {
		return worker.Report{}, errors.New("LEETCODE_SYNC_URL is required")
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
	if err != nil {
		return worker.Report{}, err
	}
	req.Header.Set("Accept", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return worker.Report{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return worker.Report{}, fmt.Errorf("leetcode catalog returned %d", res.StatusCode)
	}
	var payload struct {
		Problems []Problem `json:"problems"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(nil, res.Body, 8<<20))
	if err := dec.Decode(&payload); err != nil {
		return worker.Report{}, err
	}
	report := worker.Report{Fetched: len(payload.Problems)}
	for _, p := range payload.Problems {
		if p.Number <= 0 || p.Title == "" || (p.Difficulty != "easy" && p.Difficulty != "medium" && p.Difficulty != "hard") {
			report.Failed++
			continue
		}
		if c.Sink == nil {
			report.Updated++
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
