package postgres

import (
	"context"
	"database/sql"
	"fmt"

	api "github.com/ehazlett/flow/api/services/workflows/v1"
	"github.com/sirupsen/logrus"
)

func (p *Postgres) GetNextQueueWorkflow(ctx context.Context, queueType string, scope *api.ProcessorScope) (*api.Workflow, error) {
	p.queueLock.Lock()
	defer p.queueLock.Unlock()

	// check scope
	if scope == nil {
		scope = &api.ProcessorScope{
			Scope: &api.ProcessorScope_Global{
				Global: true,
			},
		}
	}

	var rows *sql.Rows
	switch v := scope.Scope.(type) {
	case *api.ProcessorScope_Global:
		r, err := p.db.QueryContext(ctx, "SELECT workflow FROM queue WHERE workflow->>'type' = $1 AND workflow->>'status' = 'PENDING' ORDER BY workflow->>'priority' desc, workflow->>'createdAt' asc;", queueType)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, nil
			}
			return nil, err
		}
		rows = r
	case *api.ProcessorScope_Namespace:
		r, err := p.db.QueryContext(ctx, "SELECT workflow FROM queue WHERE workflow->>'namespace' = $1 AND workflow->>'type' = $2 AND workflow->>'status' = 'PENDING' ORDER BY workflow->>'priority' desc, workflow->>'createdAt' asc;", v.Namespace, queueType)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, nil
			}
			return nil, err
		}
		rows = r
	}

	var workflow *api.Workflow

	for rows.Next() {
		var w *api.Workflow
		if err := rows.Scan(&w); err != nil {
			return nil, err
		}
		// get workflow inputs and check status
		switch v := w.Input.(type) {
		case *api.Workflow_Workflows:
			ready := false
			// TODO: better sql query to get input workflow status
		INPUT_CHECK:
			for _, x := range v.Workflows.WorkflowInputs {
				logrus.Debugf("queue: checking dependent input workflow %s", x.ID)
				var iw *api.Workflow
				if err := p.db.QueryRowContext(ctx, "SELECT workflow FROM workflows WHERE workflow @> json_build_object('id', $1::text, 'namespace', $2::text)::jsonb;", x.ID, x.Namespace).Scan(&iw); err != nil {
					if err == sql.ErrNoRows {
						return nil, fmt.Errorf("invalid input workflow %s", x.ID)
					}
					return nil, err
				}

				logrus.Debugf("%s=%s", iw.ID, iw.Status)

				switch iw.Status {
				case api.WorkflowStatus_ERROR:
					return nil, fmt.Errorf("input workflow %s resulted in error", iw.ID)
				case api.WorkflowStatus_COMPLETE:
					ready = true
				default:
					logrus.Debugf("input workflow %s not ready (%s)", iw.ID, iw.Status)
					ready = false
					break INPUT_CHECK
				}
			}
			if ready {
				workflow = w
				// stop iteration
				if err := rows.Close(); err != nil {
					return nil, err
				}
				break
			}
		default:
			workflow = w
			// stop iteration
			if err := rows.Close(); err != nil {
				return nil, err
			}
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// if no workflows are returned; return nil
	if workflow == nil {
		return nil, nil
	}

	logrus.Debugf("sending workflow %s to processor", workflow.ID)
	workflow.Status = api.WorkflowStatus_WAITING

	// update status to prevent sending to multiple processors
	if _, err := p.db.ExecContext(ctx, "UPDATE queue set workflow = ($1) WHERE workflow->>'id' = $2", workflow, workflow.ID); err != nil {
		return nil, err
	}

	return workflow, nil
}

func (p *Postgres) CreateQueueWorkflow(ctx context.Context, workflow *api.Workflow) error {
	if _, err := p.db.Exec("INSERT INTO queue (workflow) VALUES($1);", workflow); err != nil {
		return err
	}

	return nil
}

func (p *Postgres) DeleteQueueWorkflow(ctx context.Context, id string) error {
	if _, err := p.db.ExecContext(ctx, "DELETE FROM queue WHERE workflow @> json_build_object('id', $1::text)::jsonb;", id); err != nil {
		return err
	}

	return nil
}
