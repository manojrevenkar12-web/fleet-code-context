// Package log is the structured logger every service uses.
package log

import (
	"context"
	"encoding/json"
	"os"
	"time"
)

// Info writes one structured log line.
func Info(ctx context.Context, msg string, kv ...any) { write("info", msg, kv) }

// Error writes one structured error line.
func Error(ctx context.Context, msg string, kv ...any) { write("error", msg, kv) }

func write(level, msg string, kv []any) {
	rec := map[string]any{"ts": time.Now().UTC().Format(time.RFC3339), "level": level, "msg": msg}
	for i := 0; i+1 < len(kv); i += 2 {
		if k, ok := kv[i].(string); ok {
			rec[k] = kv[i+1]
		}
	}
	_ = json.NewEncoder(os.Stderr).Encode(rec)
}
