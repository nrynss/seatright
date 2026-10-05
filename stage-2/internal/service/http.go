package service

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Result is a service outcome ready to render: an HTTP status and a JSON
// body (nil for 204 No Content).
type Result struct {
	Status int
	Body   any
}

func okResult(body any) Result { return Result{Status: http.StatusOK, Body: body} }
func created(body any) Result  { return Result{Status: http.StatusCreated, Body: body} }
func noContent() Result        { return Result{Status: http.StatusNoContent} }
func internalErr() Result      { return Result{Status: 500, Body: errorBody("internal", "internal error")} }
func malformed() Result {
	return Result{Status: 400, Body: errorBody("malformed_request", "unparseable body or wrong field type")}
}
func unauthenticated() Result {
	return Result{Status: 401, Body: errorBody("unauthenticated", "missing or invalid bearer token")}
}
func notFound(msg string) Result { return Result{Status: 404, Body: errorBody("not_found", msg)} }
func conflict(code, msg string) Result {
	return Result{Status: 409, Body: errorBody(code, msg)}
}
func validationFailed(msg string) Result {
	return Result{Status: 422, Body: errorBody("validation_failed", msg)}
}

func errorBody(code, msg string) any {
	return map[string]any{"error": map[string]any{"code": code, "message": msg}}
}

// Handler returns the full HTTP router. All specified stage-1 routes are
// live, including the idempotent creation and move batches.
func (s *Service) Handler() http.Handler {
	return http.HandlerFunc(s.serve)
}

func (s *Service) serve(w http.ResponseWriter, r *http.Request) {
	path, method := r.URL.Path, r.Method
	var res Result
	switch {
	case method == http.MethodGet && path == "/health":
		res = okResult(map[string]any{"status": "ok"})
	case method == http.MethodPost && path == "/_test/reset":
		res = s.Reset(readBody(r))
	case method == http.MethodGet && path == "/_test/export":
		res = s.Export()
	case method == http.MethodPost && path == "/_test/import":
		res = s.Import(readBody(r))
	case method == http.MethodPost && path == "/auth/signup":
		res = s.Signup(readBody(r))
	case method == http.MethodPost && path == "/auth/login":
		res = s.Login(readBody(r))
	case method == http.MethodGet && path == "/restaurants":
		res = s.ListRestaurants()
	case method == http.MethodGet && strings.HasPrefix(path, "/restaurants/"):
		res = s.restaurantRoute(path)
	case method == http.MethodGet && path == "/availability":
		res = s.Availability(r.URL.Query())
	case method == http.MethodGet && path == "/reservations":
		res = s.ListReservations(bearerTokenString(r))
	case method == http.MethodPost && path == "/reservations":
		res = s.CreateReservation(bearerTokenString(r), r.Header.Get("Idempotency-Key"), readBody(r))
	case strings.HasPrefix(path, "/reservations/"):
		res = s.reservationRoute(r, path)
	case path == "/reservation-moves" && method == http.MethodPost:
		res = s.MoveReservations(bearerTokenString(r), r.Header.Get("Idempotency-Key"), readBody(r))
	case method == http.MethodGet && isPageRoute(path):
		s.serveStatic(w, r)
		return
	case method == http.MethodGet:
		if s.serveStaticFile(w, r) {
			return
		}
		res = notFound("unknown path")
	default:
		res = notFound("unknown path")
	}
	writeResult(w, res)
}

// reservationRoute dispatches GET/PATCH on /reservations/{reference} and POST
// on /reservations/{reference}/cancel. Deeper or malformed paths are 404.
func (s *Service) reservationRoute(r *http.Request, path string) Result {
	rest := strings.TrimPrefix(path, "/reservations/")
	if rest == "" {
		return notFound("unknown path")
	}
	parts := strings.Split(rest, "/")
	reference := parts[0]
	token := bearerTokenString(r)
	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		return s.GetReservation(token, reference)
	case len(parts) == 1 && r.Method == http.MethodPatch:
		return s.PatchReservation(token, reference, readBody(r))
	case len(parts) == 2 && parts[1] == "cancel" && r.Method == http.MethodPost:
		return s.CancelReservation(token, reference)
	default:
		return notFound("unknown path")
	}
}

// restaurantRoute serves GET /restaurants/{id}; deeper paths belong to later
// stages and return not_found from this foundation.
func (s *Service) restaurantRoute(path string) Result {
	id := strings.TrimPrefix(path, "/restaurants/")
	if id == "" || strings.Contains(id, "/") {
		return notFound("unknown path")
	}
	return s.GetRestaurant(id)
}

// requireAuth enforces the bearer contract before running fn: missing,
// malformed or unknown tokens give 401 unauthenticated.
func (s *Service) requireAuth(r *http.Request, fn func() Result) Result {
	token, ok := bearerToken(r)
	if !ok {
		return unauthenticated()
	}
	if _, ok := s.Authenticate(token); !ok {
		return unauthenticated()
	}
	return fn()
}

// bearerTokenString extracts the raw bearer token, or "" when the header is
// missing or malformed. Protected methods authenticate it themselves so the
// 401 mapping stays in one place.
func bearerTokenString(r *http.Request) string {
	token, _ := bearerToken(r)
	return token
}
func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

// maxBody caps request bodies so a huge payload cannot exhaust memory.
const maxBody = 8 << 20

func readBody(r *http.Request) []byte {
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
	if err != nil {
		return nil
	}
	return raw
}

func writeResult(w http.ResponseWriter, res Result) {
	if res.Status == http.StatusNoContent {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	raw, err := json.Marshal(res.Body)
	if err != nil {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"error":{"code":"internal","message":"internal error"}}`))
		return
	}
	w.WriteHeader(res.Status)
	_, _ = w.Write(raw)
}

// ListRestaurants renders the public restaurant list in fixture order.
func (s *Service) ListRestaurants() Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]any, 0, len(s.state.Restaurants))
	for _, r := range s.state.Restaurants {
		items = append(items, map[string]any{"id": r.ID, "name": r.Name, "timezone": r.Timezone})
	}
	return okResult(map[string]any{"restaurants": items})
}

// GetRestaurant renders one restaurant in the fixture shape.
func (s *Service) GetRestaurant(id string) Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := restaurantByID(&s.state, id)
	if r == nil {
		return notFound("unknown restaurant")
	}
	hours := make([]any, 0, len(r.OpeningHours))
	for _, h := range r.OpeningHours {
		hours = append(hours, map[string]any{"weekday": h.Weekday, "opens": h.Opens, "closes": h.Closes})
	}
	tables := make([]any, 0, len(r.Tables))
	for _, t := range r.Tables {
		tables = append(tables, map[string]any{"id": t.ID, "label": t.Label, "capacity": t.Capacity})
	}
	return okResult(map[string]any{
		"id":                           r.ID,
		"name":                         r.Name,
		"timezone":                     r.Timezone,
		"slot_minutes":                 r.SlotMinutes,
		"reservation_duration_minutes": r.ReservationDurationMinutes,
		"cancellation_cutoff_minutes":  r.CancellationCutoffMinutes,
		"opening_hours":                hours,
		"tables":                       tables,
		"combinable":                   combinableView(r.Combinable),
	})
}

// combinableView renders declared pairs in declared order. Absent pairs
// render as an empty array (never null) for UI consumption.
func combinableView(pairs [][]string) []any {
	out := make([]any, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, []any{p[0], p[1]})
	}
	return out
}

// isPageRoute reports the browser screen routes served with index.html once
func isPageRoute(path string) bool {
	switch path {
	case "/", "/signup", "/login", "/lookup":
		return true
	}
	return false
}

// webDir is the WEB_DIR contract: the web build emits web/dist and the
// Docker image carries it at /app/web/dist.
func webDir() string {
	if dir := os.Getenv("WEB_DIR"); dir != "" {
		return dir
	}
	return "web/dist"
}

func (s *Service) serveStatic(w http.ResponseWriter, r *http.Request) {
	index := filepath.Join(webDir(), "index.html")
	if _, err := os.Stat(index); err != nil {
		writeResult(w, notFound("web build is not available"))
		return
	}
	http.ServeFile(w, r, index)
}

// serveStaticFile serves a bundled asset path when the web build exists and
// the file is inside it. It reports whether it handled the request; API
// misses are never answered with HTML.
func (s *Service) serveStaticFile(w http.ResponseWriter, r *http.Request) bool {
	dir := webDir()
	cleaned := filepath.Clean("/" + strings.TrimPrefix(r.URL.Path, "/"))
	full := filepath.Join(dir, cleaned)
	rel, err := filepath.Rel(dir, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		return false
	}
	http.ServeFile(w, r, full)
	return true
}
