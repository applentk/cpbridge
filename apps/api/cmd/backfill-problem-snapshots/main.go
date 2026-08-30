package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/cpbridge/api/internal/db"
	"github.com/cpbridge/api/internal/platform"
	"github.com/cpbridge/api/internal/platform/atcoder"
	"github.com/cpbridge/api/internal/platform/codeforces"
	"github.com/cpbridge/api/internal/problem"
)

type problemRecord struct {
	id         string
	platform   platform.Type
	externalID string
	metadata   map[string]any
}

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	database, err := db.Connect()
	if err != nil {
		return err
	}
	defer database.Close()

	if err := db.EnsureSchema(database); err != nil {
		return err
	}

	registry := platform.NewRegistry()
	registry.Register(codeforces.New())
	registry.Register(atcoder.New())

	rows, err := database.QueryContext(ctx, `
		SELECT id, platform, external_id, metadata
		FROM problems
		WHERE jsonb_typeof(metadata->'statement') IS DISTINCT FROM 'string'
		ORDER BY id
	`)
	if err != nil {
		return fmt.Errorf("find problems without snapshots: %w", err)
	}
	defer rows.Close()

	var processed, failed int
	for rows.Next() {
		var record problemRecord
		var metadataJSON []byte
		if err := rows.Scan(&record.id, &record.platform, &record.externalID, &metadataJSON); err != nil {
			return fmt.Errorf("read problem needing snapshot: %w", err)
		}
		if err := json.Unmarshal(metadataJSON, &record.metadata); err != nil {
			return fmt.Errorf("decode metadata for problem %s: %w", record.id, err)
		}
		if record.metadata == nil {
			record.metadata = map[string]any{}
		}

		if err := backfillProblem(ctx, database, registry, record); err != nil {
			failed++
			log.Printf("snapshot failed for %s (%s/%s): %v", record.id, record.platform, record.externalID, err)
			continue
		}
		processed++
		log.Printf("snapshotted %s (%s/%s)", record.id, record.platform, record.externalID)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate problems needing snapshots: %w", err)
	}
	if failed > 0 {
		return fmt.Errorf("problem snapshot backfill failed: %d failed, %d completed", failed, processed)
	}

	log.Printf("problem snapshot backfill complete: %d completed, 0 failed", processed)
	return nil
}

func backfillProblem(ctx context.Context, database *sql.DB, registry *platform.Registry, record problemRecord) error {
	adapter, err := registry.Get(record.platform)
	if err != nil {
		return err
	}

	statement, err := adapter.GetStatement(ctx, record.externalID)
	if err != nil {
		return fmt.Errorf("fetch statement: %w", err)
	}
	normalized := &platform.NormalizedProblem{
		Platform:   record.platform,
		ExternalID: record.externalID,
		Metadata:   record.metadata,
	}
	if err := problem.SnapshotStatement(normalized, statement); err != nil {
		return err
	}

	metadataJSON, err := json.Marshal(normalized.Metadata)
	if err != nil {
		return fmt.Errorf("encode snapshot: %w", err)
	}
	result, err := database.ExecContext(ctx, `
		UPDATE problems
		SET metadata = $1, updated_at = NOW()
		WHERE id = $2
		  AND jsonb_typeof(metadata->'statement') IS DISTINCT FROM 'string'
	`, metadataJSON, record.id)
	if err != nil {
		return fmt.Errorf("save snapshot: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil {
		return fmt.Errorf("confirm snapshot save: %w", err)
	} else if affected != 1 {
		return errors.New("problem was changed or removed while backfilling")
	}
	return nil
}
