package logging

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/rs/zerolog"
)

func TestRequestLoggerLogsOneLinePerRequest(t *testing.T) {
	var buf bytes.Buffer
	e := echo.New()
	e.Use(middleware.RequestID(), RequestLoggerMiddleware(zerolog.New(&buf)))
	e.GET("/ok", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") })
	e.GET("/missing", func(c *echo.Context) error { return c.String(http.StatusNotFound, "no") })
	e.GET("/boom", func(c *echo.Context) error { return c.String(http.StatusInternalServerError, "x") })

	cases := []struct {
		path      string
		status    int
		wantLevel string
	}{
		{"/ok", 200, "info"},
		{"/missing", 404, "warn"},
		{"/boom", 500, "error"},
	}
	for _, tc := range cases {
		buf.Reset()
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))

		var line map[string]any
		if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
			t.Fatalf("%s: log output %q is not one JSON line: %v", tc.path, buf.String(), err)
		}
		if line["level"] != tc.wantLevel || line["uri"] != tc.path || line["status"] != float64(tc.status) {
			t.Errorf("%s: logged %v, want level %s, status %d", tc.path, line, tc.wantLevel, tc.status)
		}
		if id := rec.Header().Get(echo.HeaderXRequestID); id == "" || line["request_id"] != id {
			t.Errorf("%s: request_id %v, want the response's X-Request-ID %q", tc.path, line["request_id"], id)
		}
	}
}

func TestNewFallsBackToInfoOnBadLevel(t *testing.T) {
	defer zerolog.SetGlobalLevel(zerolog.GlobalLevel())

	New("debug", false)
	if zerolog.GlobalLevel() != zerolog.DebugLevel {
		t.Fatalf("level = %s, want debug", zerolog.GlobalLevel())
	}
	New("not-a-level", false)
	if zerolog.GlobalLevel() != zerolog.InfoLevel {
		t.Fatalf("level = %s, want info for an invalid setting", zerolog.GlobalLevel())
	}
}
