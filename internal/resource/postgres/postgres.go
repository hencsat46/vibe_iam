package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	resservice "temp/internal/resource/service"
	"temp/internal/pkg/logger"
)

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type postgres struct {
	pgxpool *pgxpool.Pool
	logger  *logger.Logger
}

func NewPostgres(pgxpool *pgxpool.Pool, log *logger.Logger) resservice.IPostgres {
	return &postgres{
		pgxpool: pgxpool,
		logger:  log.WithContext("postgres", "resource"),
	}
}

func (p *postgres) getQuerier(_ context.Context) querier {
	return p.pgxpool
}
