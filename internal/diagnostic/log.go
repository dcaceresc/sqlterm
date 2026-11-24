package diagnostic

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const errorLogName = "sqlterm-errors.log"

var errorLogMu sync.Mutex

// LogError agrega el error con secretos ocultos junto al archivo de configuración.
// El llamador puede ignorar los fallos del registro para que nunca sustituyan
// el error original de la operación.
func LogError(configPath, operation string, err error, secrets ...string) error {
	if err == nil {
		return nil
	}
	redactedErr := Redact(err, secrets...)
	record := fmt.Sprintf("timestamp=%s operation=%s\n%s\n\n", time.Now().UTC().Format(time.RFC3339Nano), operation, redactedErr.Error())
	logPath := filepath.Join(filepath.Dir(configPath), errorLogName)

	errorLogMu.Lock()
	defer errorLogMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(logPath), 0700); err != nil {
		return err
	}
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.WriteString(record)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
