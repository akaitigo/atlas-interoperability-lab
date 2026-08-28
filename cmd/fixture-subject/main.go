package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type event struct {
	Subject string `json:"subject"`
	Name    string `json:"name"`
	TraceID string `json:"trace_id"`
	Detail  string `json:"detail"`
}

type state struct {
	mu       sync.Mutex
	role     string
	peerURL  string
	token    string
	admin    string
	fault    bool
	messages map[string]json.RawMessage
	events   []event
}

func main() {
	role := flag.String("role", "", "source または sink")
	listen := flag.String("listen", "127.0.0.1:0", "listen address")
	peer := flag.String("peer", "", "sink base URL")
	flag.Parse()
	if *role != "source" && *role != "sink" {
		log.Fatal("-role は source または sink です")
	}
	s := &state{role: *role, peerURL: strings.TrimRight(*peer, "/"), token: os.Getenv("ATLAS_FIXTURE_TOKEN"), admin: os.Getenv("ATLAS_FIXTURE_ADMIN_TOKEN"), messages: map[string]json.RawMessage{}}
	if s.token == "" || s.admin == "" {
		log.Fatal("fixture credential が未設定です")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.health)
	mux.HandleFunc("/state", s.readState)
	mux.HandleFunc("/telemetry", s.telemetry)
	if *role == "source" {
		mux.HandleFunc("/invoke", s.invoke)
		mux.HandleFunc("/downstream-state", s.downstreamState)
		mux.HandleFunc("/admin/fault", s.proxyFault)
	} else {
		mux.HandleFunc("/messages", s.receive)
		mux.HandleFunc("/admin/fault", s.setFault)
	}
	server := &http.Server{Addr: *listen, Handler: mux, ReadHeaderTimeout: 2 * time.Second}
	log.Printf("fixture role=%s listen=%s", *role, *listen)
	log.Fatal(server.ListenAndServe())
}

func (s *state) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"status": "ready", "subject": s.role})
}

func (s *state) readState(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, 200, map[string]any{"subject": s.role, "message_count": len(s.messages), "fault": s.fault})
}

func (s *state) record(name, traceID, detail string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event{Subject: s.role, Name: name, TraceID: traceID, Detail: detail})
}

func (s *state) telemetry(w http.ResponseWriter, r *http.Request) {
	traceID := r.URL.Query().Get("trace_id")
	s.mu.Lock()
	items := append([]event(nil), s.events...)
	s.mu.Unlock()
	filtered := make([]event, 0, len(items))
	for _, item := range items {
		if traceID == "" || item.TraceID == traceID {
			filtered = append(filtered, item)
		}
	}
	if s.role == "source" && s.peerURL != "" {
		var downstream struct {
			Events []event `json:"events"`
		}
		if code, err := s.peerJSON(http.MethodGet, "/telemetry?trace_id="+traceID, nil, &downstream); err == nil && code == 200 {
			filtered = append(filtered, downstream.Events...)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].Subject == filtered[j].Subject {
			return filtered[i].Name < filtered[j].Name
		}
		return filtered[i].Subject < filtered[j].Subject
	})
	writeJSON(w, 200, map[string]any{"trace_id": traceID, "events": filtered, "event_count": len(filtered)})
}

func (s *state) invoke(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r, s.token) {
		s.record("identity.rejected", r.Header.Get("X-Trace-ID"), "invalid bearer")
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	var body struct {
		SchemaVersion int             `json:"schema_version"`
		MessageID     string          `json:"message_id"`
		Payload       json.RawMessage `json:"payload"`
	}
	if err := decodeJSON(r.Body, &body); err != nil {
		writeJSON(w, 400, map[string]any{"error": "invalid_json"})
		return
	}
	traceID := r.Header.Get("X-Trace-ID")
	if traceID == "" {
		sum := sha256.Sum256([]byte(body.MessageID))
		traceID = hex.EncodeToString(sum[:8])
	}
	if body.SchemaVersion != 1 {
		s.record("compatibility.rejected", traceID, "unsupported schema")
		writeJSON(w, 422, map[string]any{"error": "unsupported_schema", "supported": []int{1}, "trace_id": traceID})
		return
	}
	message := map[string]any{"schema_version": 1, "message_id": body.MessageID, "payload": body.Payload, "trace_id": traceID}
	var result map[string]any
	code, err := s.peerJSON(http.MethodPost, "/messages", message, &result)
	if err != nil || code != 200 {
		s.record("failure.propagated", traceID, "sink unavailable")
		writeJSON(w, 502, map[string]any{"error": "downstream_unavailable", "trace_id": traceID})
		return
	}
	s.record("communication.completed", traceID, "message delivered")
	writeJSON(w, 200, map[string]any{"status": result["status"], "message_id": body.MessageID, "trace_id": traceID, "schema_version": 1})
}

func (s *state) receive(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r, s.token) {
		writeJSON(w, 403, map[string]any{"error": "forbidden"})
		return
	}
	var body struct {
		SchemaVersion int             `json:"schema_version"`
		MessageID     string          `json:"message_id"`
		Payload       json.RawMessage `json:"payload"`
		TraceID       string          `json:"trace_id"`
	}
	if err := decodeJSON(r.Body, &body); err != nil {
		writeJSON(w, 400, map[string]any{"error": "invalid_json"})
		return
	}
	s.mu.Lock()
	fault := s.fault
	if !fault && body.SchemaVersion == 1 {
		s.messages[body.MessageID] = append(json.RawMessage(nil), body.Payload...)
	}
	s.mu.Unlock()
	if fault {
		s.record("failure.injected", body.TraceID, "synthetic isolated fault")
		writeJSON(w, 503, map[string]any{"error": "synthetic_fault"})
		return
	}
	if body.SchemaVersion != 1 {
		writeJSON(w, 422, map[string]any{"error": "unsupported_schema"})
		return
	}
	s.record("messaging.accepted", body.TraceID, body.MessageID)
	writeJSON(w, 200, map[string]any{"status": "accepted", "message_id": body.MessageID})
}

func (s *state) setFault(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r, s.admin) {
		writeJSON(w, 403, map[string]any{"error": "forbidden"})
		return
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := decodeJSON(r.Body, &body); err != nil {
		writeJSON(w, 400, map[string]any{"error": "invalid_json"})
		return
	}
	s.mu.Lock()
	s.fault = body.Enabled
	s.mu.Unlock()
	s.record("recovery.state", r.Header.Get("X-Trace-ID"), fmt.Sprintf("fault=%t", body.Enabled))
	writeJSON(w, 200, map[string]any{"fault": body.Enabled})
}

func (s *state) proxyFault(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r, s.admin) {
		writeJSON(w, 403, map[string]any{"error": "forbidden"})
		return
	}
	var raw json.RawMessage
	data, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": "invalid_body"})
		return
	}
	raw = data
	var result map[string]any
	code, err := s.peerJSONWithToken(http.MethodPost, "/admin/fault", raw, s.admin, &result)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "downstream_unavailable"})
		return
	}
	writeJSON(w, code, result)
}

func (s *state) downstreamState(w http.ResponseWriter, _ *http.Request) {
	var result map[string]any
	code, err := s.peerJSON(http.MethodGet, "/state", nil, &result)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "downstream_unavailable"})
		return
	}
	writeJSON(w, code, result)
}

func (s *state) peerJSON(method, path string, body any, out any) (int, error) {
	return s.peerJSONWithToken(method, path, body, s.token, out)
}

func (s *state) peerJSONWithToken(method, path string, body any, token string, out any) (int, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, s.peerURL+path, reader)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if out != nil {
		if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out); err != nil {
			return resp.StatusCode, err
		}
	}
	return resp.StatusCode, nil
}

func (s *state) authorized(r *http.Request, token string) bool {
	return r.Header.Get("Authorization") == "Bearer "+token
}

func decodeJSON(r io.Reader, out any) error {
	decoder := json.NewDecoder(io.LimitReader(r, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(out)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
