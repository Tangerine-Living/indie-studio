// Package logging initialises a rotating-by-day log file under the user's
// cache directory and provides a panic recovery helper for Wails bindings.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"pixelbench/internal/apperr"
)

var (
	once    sync.Once
	logDir  string
	logFile *os.File
	initErr error
)

// Init sets up the global slog logger. It is safe to call more than once.
func Init() (string, error) {
	once.Do(func() {
		base, err := os.UserCacheDir()
		if err != nil {
			base = os.TempDir()
		}
		logDir = filepath.Join(base, "PixelBench", "logs")
		if err := os.MkdirAll(logDir, 0o755); err != nil {
			initErr = err
			return
		}
		name := fmt.Sprintf("pixelbench-%s.log", time.Now().Format("20060102"))
		f, err := os.OpenFile(filepath.Join(logDir, name), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			initErr = err
			return
		}
		logFile = f
		handler := slog.NewTextHandler(io.MultiWriter(os.Stderr, f), &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
		slog.SetDefault(slog.New(handler))
	})
	return logDir, initErr
}

// Dir returns the log directory (empty before Init succeeds).
func Dir() string { return logDir }

// Close flushes and closes the log file.
func Close() {
	if logFile != nil {
		_ = logFile.Close()
	}
}

// Info logs an informational message.
func Info(msg string, args ...any) { slog.Info(msg, args...) }

// Error logs an operation failure with its scope.
func Error(scope string, err error) {
	slog.Error("operation failed", "scope", scope, "error", err)
}

// Catch recovers from a panic inside a Wails binding and converts it into an
// application error. Use it with named return values:
//
//	func (a *App) Foo() (out T, err error) {
//	    defer logging.Catch("Foo", &err)
//	    ...
//	}
func Catch(scope string, errp *error) {
	r := recover()
	if r == nil {
		return
	}
	stack := string(debug.Stack())
	slog.Error("panic recovered", "scope", scope, "panic", fmt.Sprint(r), "stack", stack)
	*errp = apperr.New("INTERNAL", "发生内部错误，请查看日志").WithDetail(fmt.Sprint(r))
}
