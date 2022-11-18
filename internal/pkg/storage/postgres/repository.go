package postgresql

import (
	"database/sql"
	"database/sql/driver"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

type Storage struct {
	db *bun.DB
}

func NewStorage(connector driver.Connector) *Storage {
	return &Storage{
		db: bun.NewDB(sql.OpenDB(connector), pgdialect.New()),
	}
}
