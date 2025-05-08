/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package db

type SqlInterface interface {
	Query(query string, args ...any) (Rows, error)
	Ping() error
}

type Rows interface {
	Columns() ([]string, error)
	Next() bool
	Scan(dest ...any) error
	Close() error
}
