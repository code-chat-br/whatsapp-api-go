package migrations

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const Dir = "internal/database/migrations"

var createTablePattern = regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?("?[^"\s(]+"?)`)
var renameTypePattern = regexp.MustCompile(`(?i)ALTER\s+TYPE\s+("?[^"\s;]+"?)\s+RENAME\s+TO\s+("?[^"\s;]+"?)`)

func Run(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name text PRIMARY KEY,
			applied_at timestamp NOT NULL DEFAULT now()
		)
	`); err != nil {
		return fmt.Errorf("ensure schema migrations table: %w", err)
	}

	entries, err := os.ReadDir(Dir)
	if err != nil {
		return fmt.Errorf("read database migrations: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var applied bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name = $1)`, name).Scan(&applied); err != nil {
			return fmt.Errorf("check database migration %s: %w", name, err)
		}
		if applied {
			continue
		}

		content, err := os.ReadFile(filepath.Join(Dir, name))
		if err != nil {
			return fmt.Errorf("read database migration %s: %w", name, err)
		}
		if shouldSkip, err := shouldSkipCreateMigration(ctx, pool, string(content)); err != nil {
			return fmt.Errorf("check existing tables for migration %s: %w", name, err)
		} else if shouldSkip {
			if err := recordMigration(ctx, pool, name); err != nil {
				return fmt.Errorf("record skipped database migration %s: %w", name, err)
			}
			continue
		}
		if shouldSkip, err := shouldSkipAppliedRenameMigration(ctx, pool, string(content)); err != nil {
			return fmt.Errorf("check renamed types for migration %s: %w", name, err)
		} else if shouldSkip {
			if err := recordMigration(ctx, pool, name); err != nil {
				return fmt.Errorf("record skipped database migration %s: %w", name, err)
			}
			continue
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin database migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, string(content)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("execute database migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (name) VALUES ($1) ON CONFLICT (name) DO NOTHING`, name); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record database migration %s: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit database migration %s: %w", name, err)
		}
	}
	return nil
}

func shouldSkipAppliedRenameMigration(ctx context.Context, pool *pgxpool.Pool, sql string) (bool, error) {
	matches := renameTypePattern.FindAllStringSubmatch(sql, -1)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		oldName := strings.Trim(match[1], `"`)
		newName := strings.Trim(match[2], `"`)
		oldExists, err := typeExists(ctx, pool, oldName)
		if err != nil {
			return false, err
		}
		newExists, err := typeExists(ctx, pool, newName)
		if err != nil {
			return false, err
		}
		if !oldExists && newExists {
			return true, nil
		}
	}
	return false, nil
}

func shouldSkipCreateMigration(ctx context.Context, pool *pgxpool.Pool, sql string) (bool, error) {
	tableNames := createTableNames(sql)
	for _, tableName := range tableNames {
		exists, err := tableExists(ctx, pool, tableName)
		if err != nil {
			return false, err
		}
		if exists {
			return true, nil
		}
	}
	return false, nil
}

func createTableNames(sql string) []string {
	matches := createTablePattern.FindAllStringSubmatch(sql, -1)
	tableNames := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		tableName := strings.Trim(match[1], `"`)
		if _, ok := seen[tableName]; ok {
			continue
		}
		seen[tableName] = struct{}{}
		tableNames = append(tableNames, tableName)
	}
	return tableNames
}

func tableExists(ctx context.Context, pool *pgxpool.Pool, tableName string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = ANY (current_schemas(false))
			  AND table_name = $1
		)
	`, tableName).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func typeExists(ctx context.Context, pool *pgxpool.Pool, typeName string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM pg_type t
			JOIN pg_namespace n ON n.oid = t.typnamespace
			WHERE n.nspname = ANY (current_schemas(false))
			  AND t.typname = $1
		)
	`, typeName).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func recordMigration(ctx context.Context, pool *pgxpool.Pool, name string) error {
	_, err := pool.Exec(ctx, `INSERT INTO schema_migrations (name) VALUES ($1) ON CONFLICT (name) DO NOTHING`, name)
	return err
}
