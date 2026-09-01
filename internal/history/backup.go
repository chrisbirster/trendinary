package history

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// Backup writes a transactionally consistent SQLite snapshot using VACUUM
// INTO. It is used by the Fly/R2 backup workflow rather than copying WAL files.
func (s *Store) Backup(ctx context.Context, destination string) error {
	if s == nil || s.db == nil { return fmt.Errorf("history store is not configured") }
	if strings.TrimSpace(destination)=="" { return fmt.Errorf("backup destination is required") }
	_ = os.Remove(destination)
	escaped:=strings.ReplaceAll(destination,"'","''")
	if _,err:=s.db.ExecContext(ctx,"VACUUM INTO '"+escaped+"'");err!=nil{return fmt.Errorf("sqlite backup: %w",err)}
	return nil
}
