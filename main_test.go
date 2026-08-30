package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchEmbedsBeforeQuery(t *testing.T) {
	cases := []struct {
		query string
		want  []string
	}{{"release rollback", []string{"/v1/embeddings", "/v1/vector/query"}}}
	for _, tc := range cases {
		seen := []string{}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seen = append(seen, r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/v1/embeddings" {
				w.Write([]byte(`{"ok":true,"data":{"data":[{"embedding":[0.1,0.2]}]}}`))
				return
			}
			w.Write([]byte(`{"ok":true,"data":{"matches":[{"id":"release-1"}]}}`))
		}))
		c := &InfraiClient{BaseURL: srv.URL, Key: "test", HTTP: srv.Client()}
		s := &SearchService{client: c, collection: "devtools-events"}
		got, err := s.Search(tc.query)
		srv.Close()
		if err != nil {
			t.Fatal(err)
		}
		if string(got) == "" || len(seen) != len(tc.want) || seen[0] != tc.want[0] || seen[1] != tc.want[1] {
			t.Fatalf("unexpected search flow: %v %s", seen, got)
		}
	}
}

func TestCollectionLifecycle(t *testing.T) {
	var methods []string
	var collections []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		var body struct {
			Collection string `json:"collection"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		collections = append(collections, body.Collection)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"data":{}}`))
	}))
	defer srv.Close()

	c := &InfraiClient{BaseURL: srv.URL, Key: "test", HTTP: srv.Client()}
	if err := c.CreateCollection("temporary-events", 2); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteCollection("temporary-events"); err != nil {
		t.Fatal(err)
	}
	if len(methods) != 2 || methods[0] != http.MethodPost || methods[1] != http.MethodDelete {
		t.Fatalf("unexpected methods: %v", methods)
	}
	if collections[0] != "temporary-events" || collections[1] != "temporary-events" {
		t.Fatalf("collection lifecycle mismatch: %v", collections)
	}
}
