package knowledge

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"
)

type indexJob struct {
	id       uint64
	entryID  uint64
	classID  uint64
	hash     string
	strategy Strategy
	attempts int
}

func contentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func (s *Service) Backfill(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `INSERT IGNORE INTO knowledge_index_jobs
		(entry_id,class_id,content_hash,index_version,strategy_json,status)
		SELECT e.id,e.class_id,SHA2(e.content,256),?, '{"mode":"auto"}','pending'
		FROM knowledge_entries e LEFT JOIN knowledge_index_jobs j
		ON j.entry_id=e.id AND j.index_version=? WHERE j.id IS NULL`, s.cfg.IndexVersion, s.cfg.IndexVersion)
	return err
}

func (s *Service) RequeueAll(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE knowledge_chunks SET index_status='pending' WHERE index_version=?`, s.cfg.IndexVersion); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE knowledge_index_jobs SET status='pending', attempts=0,
		leased_until=NULL, next_attempt_at=UTC_TIMESTAMP(6), last_error=NULL WHERE index_version=?`, s.cfg.IndexVersion)
	return err
}

func (s *Service) RebuildAll(ctx context.Context) error {
	if err := s.Backfill(ctx); err != nil { return err }
	if err := s.vectors.RecreateCollection(ctx); err != nil { return err }
	return s.RequeueAll(ctx)
}

func (s *Service) Run(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		if err := s.runOnce(ctx); err != nil && ctx.Err() == nil {
			log.Printf("knowledge index worker: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) runOnce(ctx context.Context) error {
	if err := s.Backfill(ctx); err != nil {
		return err
	}
	created, err := s.vectors.EnsureCollection(ctx)
	if err != nil {
		return err
	}
	if created {
		if err := s.RequeueAll(ctx); err != nil {
			return err
		}
	}
	job, err := s.claim(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := s.process(ctx, job); err != nil {
		if failErr := s.fail(ctx, job, err); failErr != nil {
			return fmt.Errorf("index job %d failed: %v; state update: %w", job.id, err, failErr)
		}
		return fmt.Errorf("index job %d failed: %w", job.id, err)
	}
	return nil
}

func (s *Service) claim(ctx context.Context) (indexJob, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return indexJob{}, err
	}
	defer tx.Rollback()
	var job indexJob
	var raw []byte
	err = tx.QueryRowContext(ctx, `SELECT id,entry_id,class_id,content_hash,strategy_json,attempts
		FROM knowledge_index_jobs WHERE index_version=? AND
		((status='pending' AND next_attempt_at<=UTC_TIMESTAMP(6)) OR
		(status='processing' AND leased_until<UTC_TIMESTAMP(6)))
		ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED`, s.cfg.IndexVersion).
		Scan(&job.id, &job.entryID, &job.classID, &job.hash, &raw, &job.attempts)
	if err != nil {
		return indexJob{}, err
	}
	if err := json.Unmarshal(raw, &job.strategy); err != nil {
		return indexJob{}, err
	}
	job.attempts++
	_, err = tx.ExecContext(ctx, `UPDATE knowledge_index_jobs SET status='processing',attempts=?,
		leased_until=DATE_ADD(UTC_TIMESTAMP(6), INTERVAL 5 MINUTE) WHERE id=?`, job.attempts, job.id)
	if err != nil {
		return indexJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return indexJob{}, err
	}
	return job, nil
}

func (s *Service) process(ctx context.Context, job indexJob) error {
	var body string
	var materialID uint64
	err := s.db.QueryRowContext(ctx, `SELECT e.content,e.material_id FROM knowledge_entries e
		JOIN materials m ON m.id=e.material_id AND m.class_id=e.class_id
		WHERE e.id=? AND e.class_id=?`, job.entryID, job.classID).Scan(&body, &materialID)
	if err != nil {
		return err
	}
	if contentHash(body) != job.hash {
		return errors.New("source body changed since enqueue")
	}
	chunks, err := Split(body, job.strategy)
	if err != nil {
		return err
	}
	if err := s.vectors.DeleteEntry(ctx, job.classID, job.entryID); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM knowledge_chunks WHERE entry_id=? AND index_version=?`, job.entryID, s.cfg.IndexVersion); err != nil {
		return err
	}
	for _, chunk := range chunks {
		if _, err := s.db.ExecContext(ctx, `UPDATE knowledge_index_jobs SET leased_until=DATE_ADD(UTC_TIMESTAMP(6), INTERVAL 5 MINUTE) WHERE id=? AND status='processing'`, job.id); err != nil {
			return err
		}
		result, err := s.db.ExecContext(ctx, `INSERT INTO knowledge_chunks
			(entry_id,material_id,class_id,chunk_index,chunk_text,start_offset,end_offset,offset_basis,content_hash,index_version,index_status)
			VALUES (?,?,?,?,?,?,?,?,?,?,'pending')`, job.entryID, materialID, job.classID,
			chunk.Index, chunk.Text, chunk.Start, chunk.End, chunk.OffsetBasis, job.hash, s.cfg.IndexVersion)
		if err != nil {
			return err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		vector, err := s.gateway.Embed(ctx, chunk.Text)
		if err != nil {
			return err
		}
		if err := s.vectors.Upsert(ctx, uint64(id), job.classID, materialID, job.entryID, chunk.Index, s.cfg.IndexVersion, vector); err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE knowledge_chunks SET index_status='ready' WHERE id=?`, id); err != nil {
			return err
		}
	}
	_, err = s.db.ExecContext(ctx, `UPDATE knowledge_index_jobs SET status='ready', leased_until=NULL,last_error=NULL WHERE id=?`, job.id)
	return err
}

func (s *Service) fail(ctx context.Context, job indexJob, reason error) error {
	status := "pending"
	if job.attempts >= s.cfg.IndexRetryMax {
		status = "failed"
	}
	backoff := s.cfg.IndexRetry
	for i := 1; i < job.attempts && backoff < time.Hour; i++ {
		backoff *= 2
	}
	if backoff > time.Hour {
		backoff = time.Hour
	}
	message := reason.Error()
	if len(message) > 255 {
		message = message[:255]
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE knowledge_chunks SET index_status='failed' WHERE entry_id=? AND index_version=?`, job.entryID, s.cfg.IndexVersion); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE knowledge_index_jobs SET status=?,leased_until=NULL,
		next_attempt_at=?,last_error=? WHERE id=?`, status, time.Now().UTC().Add(backoff), message, job.id)
	return err
}
