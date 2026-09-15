package following

import "github.com/chrisbirster/trendinary/internal/history"

func (s *Store) externallyManagedRuntime() bool {
	return s != nil && history.IsExternallyManagedDB(s.db)
}
