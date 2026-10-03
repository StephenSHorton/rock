// Package serve is the loopback workspace HTTP API. Clients that share a cwd
// share the session store. Events go out as SSE.
package serve

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/StephenSHorton/rock/internal/harness"
	"github.com/StephenSHorton/rock/internal/session"
)

type Factory func(cwd string) (*harness.Harness, error)

type Server struct {
	Factory Factory
	mu      sync.Mutex
	live    map[string]*live
}

type live struct {
	sess *session.Session
	h    *harness.Harness
	subs map[chan harness.Event]struct{}
}

func New(f Factory) *Server {
	return &Server{Factory: f, live: map[string]*live{}}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok\n")
	})
	mux.HandleFunc("POST /v1/sessions", s.create)
	mux.HandleFunc("GET /v1/sessions/{id}", s.get)
	mux.HandleFunc("POST /v1/sessions/{id}/prompt", s.prompt)
	mux.HandleFunc("GET /v1/events", s.events)
	return mux
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CWD   string `json:"cwd"`
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if body.CWD == "" {
		http.Error(w, "cwd required", http.StatusBadRequest)
		return
	}
	h, err := s.Factory(body.CWD)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sess, err := session.Create(body.CWD, "", body.Title)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.mu.Lock()
	s.live[sess.Meta.ID] = &live{sess: sess, h: h, subs: map[chan harness.Event]struct{}{}}
	s.mu.Unlock()
	writeJSON(w, http.StatusCreated, map[string]any{"id": sess.Meta.ID, "cwd": sess.Meta.CWD})
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.Lock()
	l := s.live[id]
	s.mu.Unlock()
	if l == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": l.sess.Meta.ID, "title": l.sess.Meta.Title, "cwd": l.sess.Meta.CWD})
}

func (s *Server) prompt(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	l := s.live[id]
	s.mu.Unlock()
	if l == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	// The HTTP request ends with 202. The turn keeps its own context.
	go l.h.Run(context.Background(), l.sess, body.Text, func(ev harness.Event) {
		s.broadcast(id, ev)
	})
	writeJSON(w, http.StatusAccepted, map[string]any{"accepted": true})
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("session")
	s.mu.Lock()
	l := s.live[id]
	s.mu.Unlock()
	if l == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "no flush", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	ch := make(chan harness.Event, 16)
	s.mu.Lock()
	l.subs[ch] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(l.subs, ch)
		s.mu.Unlock()
	}()
	fmt.Fprintf(w, "event: ready\ndata: %s\n\n", id)
	fl.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case ev := <-ch:
			raw, _ := json.Marshal(ev)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Kind, raw)
			fl.Flush()
			if ev.Kind == harness.EvDone {
				return
			}
		}
	}
}

func (s *Server) broadcast(id string, ev harness.Event) {
	s.mu.Lock()
	l := s.live[id]
	if l == nil {
		s.mu.Unlock()
		return
	}
	subs := make([]chan harness.Event, 0, len(l.subs))
	for ch := range l.subs {
		subs = append(subs, ch)
	}
	s.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// Listen is a convenience for the CLI.
func Listen(ctx context.Context, addr string, h http.Handler) error {
	srv := &http.Server{Addr: addr, Handler: h}
	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()
	err := srv.ListenAndServe()
	if err != nil && !strings.Contains(err.Error(), "closed") {
		return err
	}
	return nil
}
