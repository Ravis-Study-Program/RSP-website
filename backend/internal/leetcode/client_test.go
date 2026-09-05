package leetcode

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type recordingSink struct{ problems []Problem }

func (s *recordingSink) Upsert(_ context.Context, problem Problem) error {
	s.problems = append(s.problems, problem)
	return nil
}

func TestSyncHandlesPartialCatalogFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.Header.Get("Content-Type"))
		}
		_, _ = w.Write([]byte(`{"data":{"problemsetQuestionList":{"questions":[{"questionId":"1","title":"Two Sum","titleSlug":"two-sum","difficulty":"Easy","premium":false,"topicTags":[{"name":"Array"}]},{"questionId":"x","title":"bad","titleSlug":"bad","difficulty":"Impossible","premium":false,"topicTags":[]}]}}}`))
	}))
	defer server.Close()
	sink := &recordingSink{}
	report, err := Client{URL: server.URL, Sink: sink}.Sync(context.Background())
	if err != nil || report.Fetched != 2 || report.Applied != 1 || report.Failed != 1 {
		t.Fatalf("report=%#v err=%v", report, err)
	}
	if len(sink.problems) != 1 || sink.problems[0].Slug != "two-sum" || sink.problems[0].Difficulty != "easy" {
		t.Fatalf("sink=%#v", sink.problems)
	}
}

func TestSyncRequiresDurableSink(t *testing.T) {
	if _, err := (Client{}).Sync(context.Background()); err == nil {
		t.Fatal("expected missing sink error")
	}
}
