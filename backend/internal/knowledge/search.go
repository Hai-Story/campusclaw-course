package knowledge

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"

	"campusclaw/internal/config"
)

const NoEvidence = "资料中未找到相关内容"

var ErrUnavailable = errors.New("retrieval dependency unavailable")

type Hit struct {
	ChunkID      uint64  `json:"chunk_id"`
	MaterialID   uint64  `json:"material_id"`
	Title        string  `json:"title"`
	OriginalName string  `json:"original_name"`
	ChunkIndex   int     `json:"chunk_index"`
	Excerpt      string  `json:"excerpt"`
	Start        int     `json:"start_offset"`
	End          int     `json:"end_offset"`
	OffsetBasis  string  `json:"offset_basis"`
	KeywordScore float64 `json:"keyword_score,omitempty"`
	VectorScore  float64 `json:"vector_score,omitempty"`
	Rank         int     `json:"rank"`
}

type SearchResponse struct {
	Hits       []Hit  `json:"hits"`
	Message    string `json:"message,omitempty"`
	IndexState string `json:"index_state"`
}

type Service struct {
	db      *sql.DB
	cfg     config.Config
	gateway *gateway
	vectors *vectorStore
}

func New(database *sql.DB, cfg config.Config) *Service {
	return &Service{db: database, cfg: cfg, gateway: newGateway(cfg), vectors: newVectorStore(cfg.QdrantURL, cfg.EmbeddingDim)}
}

type rankedCandidate struct {
	id           uint64
	keywordScore float64
	vectorScore  float64
	fusionScore  float64
}

func (s *Service) Search(ctx context.Context, classID uint64, q, mode string, limit int) (SearchResponse, error) {
	state, err := s.IndexState(ctx, classID)
	if err != nil {
		return SearchResponse{}, err
	}
	var keywords, vectors []rankedCandidate
	candidateLimit := limit * 4
	if candidateLimit < 20 {
		candidateLimit = 20
	}
	if mode == "keyword" || mode == "hybrid" {
		keywords, err = s.keyword(ctx, classID, q, candidateLimit)
		if err != nil {
			return SearchResponse{}, err
		}
	}
	if mode == "vector" || mode == "hybrid" {
		if err := s.vectors.CheckCollection(ctx); err != nil {
			return SearchResponse{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
		queryVector, err := s.gateway.Embed(ctx, q)
		if err != nil {
			return SearchResponse{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
		points, err := s.vectors.Search(ctx, classID, s.cfg.IndexVersion, queryVector, candidateLimit)
		if err != nil {
			return SearchResponse{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
		for _, point := range points {
			vectors = append(vectors, rankedCandidate{id: point.ID, vectorScore: point.Score})
		}
	}
	ordered := mergeRankings(mode, keywords, vectors)
	hits := make([]Hit, 0, limit)
	for _, candidate := range ordered {
		if len(hits) >= limit {
			break
		}
		hit, err := s.hydrate(ctx, classID, candidate.id)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return SearchResponse{}, err
		}
		hit.KeywordScore = candidate.keywordScore
		hit.VectorScore = candidate.vectorScore
		hit.Rank = len(hits) + 1
		hits = append(hits, hit)
	}
	response := SearchResponse{Hits: hits, IndexState: state}
	if len(hits) == 0 {
		response.Message = NoEvidence
	}
	return response, nil
}

func mergeRankings(mode string, keywords, vectors []rankedCandidate) []rankedCandidate {
	if mode == "keyword" {
		return keywords
	}
	if mode == "vector" {
		return vectors
	}
	merged := make(map[uint64]*rankedCandidate)
	for i, candidate := range keywords {
		entry := &rankedCandidate{id: candidate.id, keywordScore: candidate.keywordScore, fusionScore: 1 / float64(60+i+1)}
		merged[candidate.id] = entry
	}
	for i, candidate := range vectors {
		entry := merged[candidate.id]
		if entry == nil {
			entry = &rankedCandidate{id: candidate.id}
			merged[candidate.id] = entry
		}
		entry.vectorScore = candidate.vectorScore
		entry.fusionScore += 1 / float64(60+i+1)
	}
	ordered := make([]rankedCandidate, 0, len(merged))
	for _, entry := range merged {
		ordered = append(ordered, *entry)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].fusionScore == ordered[j].fusionScore {
			return ordered[i].id < ordered[j].id
		}
		return ordered[i].fusionScore > ordered[j].fusionScore
	})
	return ordered
}

func (s *Service) keyword(ctx context.Context, classID uint64, q string, limit int) ([]rankedCandidate, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, MATCH(chunk_text) AGAINST (? IN NATURAL LANGUAGE MODE) AS score
		FROM knowledge_chunks WHERE class_id=? AND index_version=? AND index_status='ready'
		AND MATCH(chunk_text) AGAINST (? IN NATURAL LANGUAGE MODE) > 0
		ORDER BY score DESC, id ASC LIMIT ?`, q, classID, s.cfg.IndexVersion, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]rankedCandidate, 0)
	for rows.Next() {
		var item rankedCandidate
		if err := rows.Scan(&item.id, &item.keywordScore); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) hydrate(ctx context.Context, classID, chunkID uint64) (Hit, error) {
	var hit Hit
	err := s.db.QueryRowContext(ctx, `SELECT kc.id, kc.material_id, m.title, m.original_name,
		kc.chunk_index, kc.chunk_text, kc.start_offset, kc.end_offset, kc.offset_basis
		FROM knowledge_chunks kc JOIN knowledge_entries e ON e.id=kc.entry_id
		JOIN materials m ON m.id=kc.material_id AND m.id=e.material_id
		WHERE kc.id=? AND kc.class_id=? AND e.class_id=? AND m.class_id=?
		AND kc.index_version=? AND kc.index_status='ready'
		AND kc.content_hash=SHA2(e.content, 256)`,
		chunkID, classID, classID, classID, s.cfg.IndexVersion).
		Scan(&hit.ChunkID, &hit.MaterialID, &hit.Title, &hit.OriginalName,
			&hit.ChunkIndex, &hit.Excerpt, &hit.Start, &hit.End, &hit.OffsetBasis)
	return hit, err
}

func (s *Service) IndexState(ctx context.Context, classID uint64) (string, error) {
	var failed, building int
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(status='failed'),0),
		COALESCE(SUM(status IN ('pending','processing')),0)
		FROM knowledge_index_jobs WHERE class_id=? AND index_version=?`, classID, s.cfg.IndexVersion).
		Scan(&failed, &building)
	if err != nil {
		return "", err
	}
	if failed > 0 {
		return "degraded", nil
	}
	if building > 0 {
		return "building", nil
	}
	return "ready", nil
}
