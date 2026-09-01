package history

import "sync"

const (
	BackendSQLite = "sqlite"
	BackendTurso  = "turso"
)

var storeBackends sync.Map

func markBackend(store *Store, backend string) {
	if store == nil {
		return
	}
	storeBackends.Store(store, backend)
}

// Backend reports which durable SQL backend owns this store. Local stores
// created by Open default to SQLite; production Turso stores are marked by
// OpenTurso.
func (s *Store) Backend() string {
	if s == nil {
		return ""
	}
	if value, ok := storeBackends.Load(s); ok {
		if backend, ok := value.(string); ok && backend != "" {
			return backend
		}
	}
	return BackendSQLite
}
