package postgres

import (
	"context"
	"database/sql"

	"github.com/ehazlett/flow"
)

func (p *Postgres) GetTotalWorkflowsCount(ctx context.Context) (uint64, error) {
	ns, err := flow.GetNamespaceFromContext(ctx)
	if err != nil {
		return 0, err
	}
	var count int
	row := p.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM workflows WHERE workflow->>'namespace' = $1", ns)
	if err := row.Scan(&count); err != nil {
		if err == sql.ErrNoRows {
			return uint64(0), nil
		}
		return uint64(0), err
	}

	return uint64(count), nil
}

func (p *Postgres) GetPendingWorkflowsCount(ctx context.Context) (uint64, error) {
	ns, err := flow.GetNamespaceFromContext(ctx)
	if err != nil {
		return 0, err
	}
	var count int
	row := p.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM queue WHERE workflow->>'namespace' = $1", ns)
	if err := row.Scan(&count); err != nil {
		if err == sql.ErrNoRows {
			return uint64(0), nil
		}
		return uint64(0), err
	}

	return uint64(count), nil
}

func (p *Postgres) GetTotalProcessorsCount(ctx context.Context) (uint64, error) {
	return uint64(0), nil
}
