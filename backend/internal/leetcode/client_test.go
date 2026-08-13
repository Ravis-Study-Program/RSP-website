package leetcode

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSyncHandlesPartialCatalogFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"problems":[{"number":1,"title":"Two Sum","difficulty":"easy"},{"number":0,"title":"bad","difficulty":"impossible"}]}`))
	}))
	defer server.Close()
	report, err := Client{URL: server.URL}.Sync(context.Background())
	if err != nil || report.Fetched != 2 || report.Updated != 1 || report.Failed != 1 {
		t.Fatalf("report=%#v err=%v", report, err)
	}
}
