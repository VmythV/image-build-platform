package storage

import "database/sql"

type Repositories struct {
	DB         *sql.DB
	DriverName string
}

func NewRepositories(store *Store) Repositories {
	if store == nil {
		return Repositories{}
	}
	return Repositories{DB: store.DB, DriverName: store.DriverName}
}
