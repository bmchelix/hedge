/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package db

import (
	"database/sql"
)

type SqlDBWrapper struct {
	Db *sql.DB
}

func (w *SqlDBWrapper) Query(query string, args ...interface{}) (Rows, error) {
	rows, err := w.Db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	return &sqlRowsWrapper{rows}, nil
}

func (w *SqlDBWrapper) Ping() error {
	return w.Db.Ping()
}

type sqlRowsWrapper struct {
	*sql.Rows
}

func (s sqlRowsWrapper) Columns() ([]string, error) {
	return s.Rows.Columns()
}

func (s sqlRowsWrapper) Next() bool {
	return s.Rows.Next()
}

func (s sqlRowsWrapper) Scan(dest ...interface{}) error {
	return s.Rows.Scan(dest...)
}

func (s sqlRowsWrapper) Close() error {
	return s.Rows.Close()
}
