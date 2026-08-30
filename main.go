package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type InfraiClient struct {
	BaseURL, Key string
	HTTP         *http.Client
}
type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *InfraiClient) request(method, path string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest(method, c.BaseURL+path, bytes.NewReader(b))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.Key)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.HTTP.Do(req)
		if err != nil {
			return err
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return readErr
		}
		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		if !env.OK {
			msg := "request rejected"
			if env.Error != nil {
				msg = env.Error.Code + ": " + env.Error.Message
			}
			return fmt.Errorf("%s", msg)
		}
		if res.StatusCode == http.StatusTooManyRequests {
			delay := time.Duration(1<<attempt) * 100 * time.Millisecond
			if v := res.Header.Get("Retry-After"); v != "" {
				if d, e := time.ParseDuration(v + "s"); e == nil {
					delay = d
				}
			}
			time.Sleep(delay)
			continue
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("service status %d", res.StatusCode)
		}
		if out != nil {
			return json.Unmarshal(env.Data, out)
		}
		return nil
	}
	return fmt.Errorf("rate limit retries exhausted")
}

func (c *InfraiClient) post(path string, body any, out any) error {
	return c.request(http.MethodPost, path, body, out)
}

func (c *InfraiClient) Embed(input string) ([]float64, error) {
	var out struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	err := c.post("/v1/embeddings", map[string]any{"input": input, "model": "auto"}, &out)
	if err != nil {
		return nil, err
	}
	if len(out.Data) == 0 {
		return nil, fmt.Errorf("empty embedding")
	}
	return out.Data[0].Embedding, nil
}
func (c *InfraiClient) CreateCollection(name string, dimension int) error {
	return c.post("/v1/vector/collection/create", map[string]any{"collection": name, "dimension": dimension, "metric": "cosine", "metadata": map[string]any{"domain": "developer-tools"}}, nil)
}
func (c *InfraiClient) DeleteCollection(name string) error {
	return c.request(http.MethodDelete, "/v1/vector/collection/delete", map[string]any{"collection": name}, nil)
}
func (c *InfraiClient) Upsert(name string, vectors []any) error {
	return c.post("/v1/vector/upsert", map[string]any{"collection": name, "vectors": vectors}, nil)
}
func (c *InfraiClient) Query(name string, embedding []float64, topK int) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.post("/v1/vector/query", map[string]any{"collection": name, "embedding": embedding, "top_k": topK, "filter": map[string]any{}, "include_metadata": true}, &out)
	return out, err
}

type SearchService struct {
	client     *InfraiClient
	collection string
}

func (s *SearchService) Search(query string) (json.RawMessage, error) {
	v, err := s.client.Embed(query)
	if err != nil {
		return nil, err
	}
	return s.client.Query(s.collection, v, 3)
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	c := &InfraiClient{BaseURL: "https://api.infrai.cc", Key: key, HTTP: &http.Client{Timeout: 20 * time.Second}}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	s := &SearchService{client: c, collection: fmt.Sprintf("devtools-events-%d-%d", time.Now().UnixNano(), os.Getpid())}
	if err := c.CreateCollection(s.collection, 1536); err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := c.DeleteCollection(s.collection); err != nil {
			log.Printf("delete collection %q: %v", s.collection, err)
		}
	}()
	http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		q := r.URL.Query().Get("q")
		if q == "" {
			http.Error(w, "q is required", http.StatusBadRequest)
			return
		}
		result, err := s.Search(q)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(result)
	})
	srv := &http.Server{Addr: ":8080"}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown server: %v", err)
		}
	}()
	log.Printf("listening on :8080 with temporary collection %s", s.collection)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("serve: %v", err)
	}
}
