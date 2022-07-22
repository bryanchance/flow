package postgres

import (
	"context"
	"database/sql"

	"github.com/ehazlett/flow"
	api "github.com/ehazlett/flow/api/services/workflows/v1"
	"github.com/sirupsen/logrus"
)

func (p *Postgres) GetWorkflows(ctx context.Context) ([]*api.Workflow, error) {
	ns, err := flow.GetNamespaceFromContext(ctx)
	if err != nil {
		return nil, err
	}
	var workflows []*api.Workflow
	rows, err := p.db.QueryContext(ctx, "SELECT workflow FROM workflows WHERE workflow->>'namespace' = $1 ORDER by workflow->>'createdAt' desc;", ns)
	if err != nil {
		if err == sql.ErrNoRows {
			return workflows, nil
		}
		return nil, err
	}

	for rows.Next() {
		var w *api.Workflow
		if err := rows.Scan(&w); err != nil {
			return nil, err
		}
		workflows = append(workflows, w)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return workflows, nil
}

func (p *Postgres) GetWorkflow(ctx context.Context, id string) (*api.Workflow, error) {
	ns, err := flow.GetNamespaceFromContext(ctx)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("querying workflow %s in ns %s", id, ns)
	var workflow *api.Workflow
	if err := p.db.QueryRowContext(ctx, "SELECT workflow FROM workflows WHERE workflow @> json_build_object('id', $1::text, 'namespace', $2::text)::jsonb;", id, ns).Scan(&workflow); err != nil {
		if err == sql.ErrNoRows {
			return nil, flow.ErrWorkflowDoesNotExist
		}
		return nil, err
	}
	return workflow, nil
}

func (p *Postgres) CreateWorkflow(ctx context.Context, workflow *api.Workflow) error {
	if _, err := p.db.Exec("INSERT INTO workflows (workflow) VALUES($1)", workflow); err != nil {
		return err
	}

	return nil
}

func (p *Postgres) UpdateWorkflow(ctx context.Context, workflow *api.Workflow) error {
	ns, err := flow.GetNamespaceFromContext(ctx)
	if err != nil {
		return err
	}
	if _, err := p.db.ExecContext(ctx, "UPDATE workflows set workflow = ($1) WHERE workflow @> json_build_object('id', $2::text, 'namespace', $3::text)::jsonb;", workflow, workflow.ID, ns); err != nil {
		return err
	}

	return nil
}

func (p *Postgres) DeleteWorkflow(ctx context.Context, id string) error {
	ns, err := flow.GetNamespaceFromContext(ctx)
	if err != nil {
		return err
	}
	if _, err := p.db.ExecContext(ctx, "DELETE FROM workflows WHERE workflow @> json_build_object('id', $1::text, 'namespace', $2::text)::jsonb;", id, ns); err != nil {
		return err
	}

	return nil
}
