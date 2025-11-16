package pg

import (
	"context"
	"database/sql"

	trmsql "github.com/avito-tech/go-transaction-manager/drivers/sql/v2"
	"github.com/avito-tech/go-transaction-manager/trm/v2"
	trmcontext "github.com/avito-tech/go-transaction-manager/trm/v2/context"
	trmmanager "github.com/avito-tech/go-transaction-manager/trm/v2/manager"

	"prservice/internal/domain"
)

var ctxGetter = trmsql.DefaultCtxGetter

type TxManager struct {
	tm trm.Manager
}

func NewTxManager(db *sql.DB) domain.UnitOfWork {
	mgr := trmmanager.Must(
		trmsql.NewDefaultFactory(db),
		trmmanager.WithCtxManager(trmcontext.DefaultManager),
	)

	return &TxManager{tm: mgr}
}

func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return m.tm.Do(ctx, fn)
}

func exec(ctx context.Context, db *sql.DB, query string, args ...any) (sql.Result, error) {
	tr := ctxGetter.DefaultTrOrDB(ctx, db)
	return tr.ExecContext(ctx, query, args...)
}

func queryRow(ctx context.Context, db *sql.DB, query string, args ...any) *sql.Row {
	tr := ctxGetter.DefaultTrOrDB(ctx, db)
	return tr.QueryRowContext(ctx, query, args...)
}

func query(ctx context.Context, db *sql.DB, query string, args ...any) (*sql.Rows, error) {
	tr := ctxGetter.DefaultTrOrDB(ctx, db)
	return tr.QueryContext(ctx, query, args...)
}
