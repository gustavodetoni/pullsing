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

func (s *Store) CreateFlag(ctx context.Context, flag domain.Flag) (domain.Flag, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Flag{}, fmt.Errorf("begin create flag tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := tx.QueryRow(ctx, `
		UPDATE environments
		SET revision = revision + 1,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING revision
	`, flag.EnvironmentID).Scan(&flag.Revision); err != nil {
		return domain.Flag{}, mapError(err)
	}

	row := tx.QueryRow(ctx, `
		INSERT INTO flags (
			environment_id, key, name, description, enabled, value_boolean, revision
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, environment_id, key, name, description, enabled, value_boolean, revision, created_at, updated_at, archived_at
	`, flag.EnvironmentID, flag.Key, flag.Name, flag.Description, flag.Enabled, flag.BoolValue, flag.Revision)

	if err := row.Scan(
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

	if err := tx.Commit(ctx); err != nil {
		return domain.Flag{}, fmt.Errorf("commit create flag tx: %w", err)
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
