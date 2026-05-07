package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gustavodetoni/pullsing/internal/application"
	"github.com/gustavodetoni/pullsing/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return pool, nil
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) GetEnvironmentByAPIKeyHash(ctx context.Context, tokenHash string) (domain.Environment, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT e.id, e.project_id, e.key, e.name, e.revision, e.created_at, e.updated_at
		FROM api_keys ak
		JOIN environments e ON e.id = ak.environment_id
		WHERE ak.token_hash = $1
		  AND ak.revoked_at IS NULL
	`, tokenHash)

	var environment domain.Environment
	if err := row.Scan(
		&environment.ID,
		&environment.ProjectID,
		&environment.Key,
		&environment.Name,
		&environment.Revision,
		&environment.CreatedAt,
		&environment.UpdatedAt,
	); err != nil {
		return domain.Environment{}, mapError(err)
	}

	return environment, nil
}

func (s *Store) GetSnapshot(ctx context.Context, environmentID int64) (application.EnvironmentSnapshot, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return application.EnvironmentSnapshot{}, fmt.Errorf("begin get snapshot tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var revision uint64
	if err := tx.QueryRow(ctx, `
		SELECT revision
		FROM environments
		WHERE id = $1
	`, environmentID).Scan(&revision); err != nil {
		return application.EnvironmentSnapshot{}, mapError(err)
	}

	rows, err := tx.Query(ctx, `
		SELECT key, enabled, value_boolean, archived_at IS NOT NULL AS archived, revision
		FROM flags
		WHERE environment_id = $1
		  AND archived_at IS NULL
		  AND revision <= $2
		ORDER BY key ASC
	`, environmentID, revision)
	if err != nil {
		return application.EnvironmentSnapshot{}, mapError(err)
	}
	defer rows.Close()

	flags := make([]application.FlagState, 0)
	for rows.Next() {
		var flag application.FlagState
		if err := rows.Scan(&flag.Key, &flag.Enabled, &flag.BoolValue, &flag.Archived, &flag.Revision); err != nil {
			return application.EnvironmentSnapshot{}, fmt.Errorf("scan snapshot flag: %w", err)
		}
		flags = append(flags, flag)
	}
	if err := rows.Err(); err != nil {
		return application.EnvironmentSnapshot{}, fmt.Errorf("iterate snapshot flags: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return application.EnvironmentSnapshot{}, fmt.Errorf("commit get snapshot tx: %w", err)
	}

	return application.EnvironmentSnapshot{
		Revision: revision,
		Flags:    flags,
	}, nil
}

func (s *Store) ListFlagStatesSince(ctx context.Context, environmentID int64, sinceRevision uint64) (application.EnvironmentFlagChanges, error) {
	var changes application.EnvironmentFlagChanges
	if err := s.pool.QueryRow(ctx, `
		SELECT revision
		FROM environments
		WHERE id = $1
	`, environmentID).Scan(&changes.CurrentRevision); err != nil {
		return application.EnvironmentFlagChanges{}, mapError(err)
	}

	rows, err := s.pool.Query(ctx, `
		SELECT key, enabled, value_boolean, archived_at IS NOT NULL AS archived, revision
		FROM flags
		WHERE environment_id = $1
		  AND revision > $2
		ORDER BY revision ASC, key ASC
	`, environmentID, sinceRevision)
	if err != nil {
		return application.EnvironmentFlagChanges{}, mapError(err)
	}
	defer rows.Close()

	for rows.Next() {
		var flag application.FlagState
		if err := rows.Scan(&flag.Key, &flag.Enabled, &flag.BoolValue, &flag.Archived, &flag.Revision); err != nil {
			return application.EnvironmentFlagChanges{}, fmt.Errorf("scan flag change: %w", err)
		}
		changes.Flags = append(changes.Flags, flag)
	}
	if err := rows.Err(); err != nil {
		return application.EnvironmentFlagChanges{}, fmt.Errorf("iterate flag changes: %w", err)
	}

	return changes, nil
}

func (s *Store) CreateProject(ctx context.Context, project domain.Project) (domain.Project, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO projects (key, name)
		VALUES ($1, $2)
		RETURNING id, key, name, created_at, updated_at
	`, project.Key, project.Name)

	if err := row.Scan(&project.ID, &project.Key, &project.Name, &project.CreatedAt, &project.UpdatedAt); err != nil {
		return domain.Project{}, mapError(err)
	}

	return project, nil
}

func (s *Store) CreateEnvironmentWithAPIKey(ctx context.Context, environment domain.Environment, tokenHash string) (domain.Environment, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Environment{}, fmt.Errorf("begin create environment tx: %w", err)
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		INSERT INTO environments (project_id, key, name)
		VALUES ($1, $2, $3)
		RETURNING id, project_id, key, name, revision, created_at, updated_at
	`, environment.ProjectID, environment.Key, environment.Name)

	if err := row.Scan(
		&environment.ID,
		&environment.ProjectID,
		&environment.Key,
		&environment.Name,
		&environment.Revision,
		&environment.CreatedAt,
		&environment.UpdatedAt,
	); err != nil {
		return domain.Environment{}, mapError(err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO api_keys (environment_id, token_hash)
		VALUES ($1, $2)
	`, environment.ID, tokenHash); err != nil {
		return domain.Environment{}, mapError(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Environment{}, fmt.Errorf("commit create environment tx: %w", err)
	}

	return environment, nil
}

func (s *Store) RotateAPIKey(ctx context.Context, environmentID int64, tokenHash string) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin rotate api key tx: %w", err)
	}
	defer tx.Rollback(ctx)

	commandTag, err := tx.Exec(ctx, `
		UPDATE environments
		SET updated_at = NOW()
		WHERE id = $1
	`, environmentID)
	if err != nil {
		return mapError(err)
	}
	if commandTag.RowsAffected() == 0 {
		return application.ErrNotFound
	}

	if _, err := tx.Exec(ctx, `
		UPDATE api_keys
		SET revoked_at = NOW()
		WHERE environment_id = $1 AND revoked_at IS NULL
	`, environmentID); err != nil {
		return mapError(err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO api_keys (environment_id, token_hash)
		VALUES ($1, $2)
	`, environmentID, tokenHash); err != nil {
		return mapError(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit rotate api key tx: %w", err)
	}

	return nil
}

func (s *Store) ListFlags(ctx context.Context, environmentID int64, includeArchived bool) ([]domain.Flag, error) {
	if err := s.ensureEnvironmentExists(ctx, environmentID); err != nil {
		return nil, err
	}

	query := `
		SELECT id, environment_id, key, name, description, enabled, value_boolean, revision, created_at, updated_at, archived_at
		FROM flags
		WHERE environment_id = $1
	`
	if !includeArchived {
		query += " AND archived_at IS NULL"
	}
	query += " ORDER BY key ASC"

	rows, err := s.pool.Query(ctx, query, environmentID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	flags := make([]domain.Flag, 0)
	for rows.Next() {
		flag, err := scanFlag(rows)
		if err != nil {
			return nil, err
		}
		flags = append(flags, flag)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate flags: %w", err)
	}

	return flags, nil
}

func (s *Store) GetFlag(ctx context.Context, environmentID, flagID int64) (domain.Flag, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, environment_id, key, name, description, enabled, value_boolean, revision, created_at, updated_at, archived_at
		FROM flags
		WHERE environment_id = $1 AND id = $2
	`, environmentID, flagID)

	flag, err := scanFlag(row)
	if err != nil {
		return domain.Flag{}, err
	}

	return flag, nil
}

func (s *Store) CreateFlag(ctx context.Context, flag domain.Flag) (domain.Flag, error) {
	row := s.pool.QueryRow(ctx, `
		WITH next_revision AS (
			UPDATE environments
			SET revision = revision + 1,
			    updated_at = NOW()
			WHERE id = $1
			RETURNING revision
		)
		INSERT INTO flags (
			environment_id, key, name, description, enabled, value_boolean, revision
		)
		SELECT $1, $2, $3, $4, $5, $6, next_revision.revision
		FROM next_revision
		RETURNING id, environment_id, key, name, description, enabled, value_boolean, revision, created_at, updated_at, archived_at
	`, flag.EnvironmentID, flag.Key, flag.Name, flag.Description, flag.Enabled, flag.BoolValue)

	created, err := scanFlag(row)
	if err != nil {
		return domain.Flag{}, err
	}

	return created, nil
}

func (s *Store) UpdateFlag(ctx context.Context, flag domain.Flag) (domain.Flag, error) {
	row := s.pool.QueryRow(ctx, `
		WITH target AS (
			SELECT id
			FROM flags
			WHERE environment_id = $1
			  AND id = $2
			  AND archived_at IS NULL
		),
		next_revision AS (
			UPDATE environments
			SET revision = revision + 1,
			    updated_at = NOW()
			WHERE id = $1
			  AND EXISTS (SELECT 1 FROM target)
			RETURNING revision
		)
		UPDATE flags
		SET name = $3,
		    description = $4,
		    enabled = $5,
		    value_boolean = $6,
		    revision = next_revision.revision,
		    updated_at = NOW()
		FROM next_revision
		WHERE flags.environment_id = $1
		  AND flags.id = $2
		  AND flags.archived_at IS NULL
		RETURNING flags.id, flags.environment_id, flags.key, flags.name, flags.description, flags.enabled, flags.value_boolean, flags.revision, flags.created_at, flags.updated_at, flags.archived_at
	`, flag.EnvironmentID, flag.ID, flag.Name, flag.Description, flag.Enabled, flag.BoolValue)

	updated, err := scanFlag(row)
	if err != nil {
		return domain.Flag{}, err
	}

	return updated, nil
}

func (s *Store) ArchiveFlag(ctx context.Context, environmentID, flagID int64) (domain.Flag, error) {
	row := s.pool.QueryRow(ctx, `
		WITH target AS (
			SELECT id
			FROM flags
			WHERE environment_id = $1
			  AND id = $2
			  AND archived_at IS NULL
		),
		next_revision AS (
			UPDATE environments
			SET revision = revision + 1,
			    updated_at = NOW()
			WHERE id = $1
			  AND EXISTS (SELECT 1 FROM target)
			RETURNING revision
		)
		UPDATE flags
		SET archived_at = NOW(),
		    revision = next_revision.revision,
		    updated_at = NOW()
		FROM next_revision
		WHERE flags.environment_id = $1
		  AND flags.id = $2
		  AND flags.archived_at IS NULL
		RETURNING flags.id, flags.environment_id, flags.key, flags.name, flags.description, flags.enabled, flags.value_boolean, flags.revision, flags.created_at, flags.updated_at, flags.archived_at
	`, environmentID, flagID)

	archived, err := scanFlag(row)
	if err != nil {
		return domain.Flag{}, err
	}

	return archived, nil
}

func (s *Store) ensureEnvironmentExists(ctx context.Context, environmentID int64) error {
	var exists bool
	if err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM environments
			WHERE id = $1
		)
	`, environmentID).Scan(&exists); err != nil {
		return mapError(err)
	}
	if !exists {
		return application.ErrNotFound
	}

	return nil
}

type flagScanner interface {
	Scan(dest ...any) error
}

func scanFlag(scanner flagScanner) (domain.Flag, error) {
	var flag domain.Flag
	flag.Type = domain.FlagTypeBool

	if err := scanner.Scan(
		&flag.ID,
		&flag.EnvironmentID,
		&flag.Key,
		&flag.Name,
		&flag.Description,
		&flag.Enabled,
		&flag.BoolValue,
		&flag.Revision,
		&flag.CreatedAt,
		&flag.UpdatedAt,
		&flag.ArchivedAt,
	); err != nil {
		return domain.Flag{}, mapError(err)
	}

	return flag, nil
}

func mapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return application.ErrNotFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return application.ErrConflict
		case "23503":
			return application.ErrNotFound
		}
	}

	if strings.Contains(err.Error(), "no rows in result set") {
		return application.ErrNotFound
	}

	return err
}
