package functions

import (
	"hedge/app-services/export-biz-data/db"
	"github.com/stretchr/testify/mock"
)

type MockSqlDB struct {
	mock.Mock
}

func (m *MockSqlDB) Query(query string, args ...interface{}) (db.Rows, error) {
	arg := m.Called(query, args)
	return arg.Get(0).(db.Rows), arg.Error(1)
}

func (m *MockSqlDB) Ping() error {
	arg := m.Called()
	return arg.Error(0)
}

type MockRows struct {
	mock.Mock
}

func (m *MockRows) Columns() ([]string, error) {
	args := m.Called()
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockRows) Next() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockRows) Scan(dest ...interface{}) error {
	args := m.Called(dest...)

	// Simulate database scan operation
	for i, d := range dest {
		// Dereference the pointer to get the actual interface{}
		if ptr, ok := d.(*interface{}); ok {
			switch i {
			case 0: // deviceName
				*ptr = "deviceName" // Assigning a string directly to the interface
				// Add cases for other columns as needed
			case 1: // deviceName
				*ptr = "someOtherColumn"
			}
		}
	}

	return args.Error(0)
}

func (m *MockRows) Close() error {
	args := m.Called()
	return args.Error(0)
}
