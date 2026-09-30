package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const collectionName = "campusclaw_chunks"

type vectorStore struct {
	baseURL string
	dim     int
	client  *http.Client
}

type vectorCandidate struct {
	ID    uint64
	Score float64
}

func newVectorStore(baseURL string, dim int) *vectorStore {
	return &vectorStore{baseURL: baseURL, dim: dim, client: &http.Client{Timeout: 15 * time.Second}}
}

func (v *vectorStore) request(ctx context.Context, method, path string, input, output any) (int, error) {
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return 0, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, v.baseURL+path, body)
	if err != nil {
		return 0, err
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("vector store unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("vector store returned HTTP %d", resp.StatusCode)
	}
	if output != nil {
		if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(output); err != nil {
			return resp.StatusCode, fmt.Errorf("decode vector response: %w", err)
		}
	}
	return resp.StatusCode, nil
}

func (v *vectorStore) collection(ctx context.Context) (int, error) {
	path := "/collections/" + collectionName
	var existing struct {
		Result struct {
			Config struct {
				Params struct {
					Vectors struct {
						Size     int    `json:"size"`
						Distance string `json:"distance"`
					} `json:"vectors"`
				} `json:"params"`
			} `json:"config"`
		} `json:"result"`
	}
	status, err := v.request(ctx, http.MethodGet, path, nil, &existing)
	if status == http.StatusNotFound {
		return status, err
	}
	if err != nil {
		return status, err
	}
	if existing.Result.Config.Params.Vectors.Size != v.dim || existing.Result.Config.Params.Vectors.Distance != "Cosine" {
		return status, errors.New("vector collection dimension or metric mismatch")
	}
	return status, nil
}

func (v *vectorStore) CheckCollection(ctx context.Context) error {
	_, err := v.collection(ctx)
	return err
}

func (v *vectorStore) EnsureCollection(ctx context.Context) (bool, error) {
	status, err := v.collection(ctx)
	if status == http.StatusNotFound {
		path := "/collections/" + collectionName
		_, err = v.request(ctx, http.MethodPut, path, map[string]any{"vectors": map[string]any{"size": v.dim, "distance": "Cosine"}}, nil)
		if err != nil {
			return false, err
		}
		for _, field := range []struct{ name, schema string }{{"class_id", "integer"}, {"index_version", "keyword"}, {"entry_id", "integer"}} {
			_, err = v.request(ctx, http.MethodPut, path+"/index", map[string]any{"field_name": field.name, "field_schema": field.schema}, nil)
			if err != nil {
				return false, err
			}
		}
		return true, nil
	}
	return false, err
}

func (v *vectorStore) Upsert(ctx context.Context, chunkID, classID, materialID, entryID uint64, chunkIndex int, version string, vector []float32) error {
	if len(vector) != v.dim {
		return errors.New("embedding dimension mismatch")
	}
	point := map[string]any{"id": chunkID, "vector": vector, "payload": map[string]any{
		"class_id": classID, "material_id": materialID, "knowledge_entry_id": entryID,
		"entry_id": entryID, "chunk_id": chunkID, "chunk_index": chunkIndex, "index_version": version,
	}}
	_, err := v.request(ctx, http.MethodPut, "/collections/"+collectionName+"/points?wait=true", map[string]any{"points": []any{point}}, nil)
	return err
}

func (v *vectorStore) DeleteEntry(ctx context.Context, classID, entryID uint64) error {
	filter := map[string]any{"must": []any{
		map[string]any{"key": "class_id", "match": map[string]any{"value": classID}},
		map[string]any{"key": "entry_id", "match": map[string]any{"value": entryID}},
	}}
	_, err := v.request(ctx, http.MethodPost, "/collections/"+collectionName+"/points/delete?wait=true", map[string]any{"filter": filter}, nil)
	return err
}

func (v *vectorStore) RecreateCollection(ctx context.Context) error {
	status, err := v.request(ctx, http.MethodDelete, "/collections/"+collectionName, nil, nil)
	if err != nil && status != http.StatusNotFound { return err }
	_, err = v.EnsureCollection(ctx)
	return err
}

func (v *vectorStore) Search(ctx context.Context, classID uint64, version string, vector []float32, limit int) ([]vectorCandidate, error) {
	if len(vector) != v.dim {
		return nil, errors.New("embedding dimension mismatch")
	}
	filter := map[string]any{"must": []any{
		map[string]any{"key": "class_id", "match": map[string]any{"value": classID}},
		map[string]any{"key": "index_version", "match": map[string]any{"value": version}},
	}}
	var result struct {
		Result []struct {
			ID    uint64  `json:"id"`
			Score float64 `json:"score"`
		} `json:"result"`
	}
	_, err := v.request(ctx, http.MethodPost, "/collections/"+collectionName+"/points/search", map[string]any{
		"vector": vector, "filter": filter, "score_threshold": 0.35, "limit": limit, "with_payload": false,
	}, &result)
	if err != nil {
		return nil, err
	}
	candidates := make([]vectorCandidate, 0, len(result.Result))
	for _, point := range result.Result {
		if point.Score >= 0.35 {
			candidates = append(candidates, vectorCandidate{ID: point.ID, Score: point.Score})
		}
	}
	return candidates, nil
}
