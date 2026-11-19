package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

type callRequest struct {
	Method  string   `json:"method"`
	Session string   `json:"session"`
	Args    []string `json:"args"`
}

type lifecycleRequest struct {
	Hook    string `json:"hook"`
	Session string `json:"session"`
}

func (r *pluginRegistry) listHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, r.list())
}

func (r *pluginRegistry) pluginHandler(w http.ResponseWriter, req *http.Request) {
	rest := strings.TrimPrefix(req.URL.Path, "/api/plugins/")
	parts := strings.Split(rest, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, req)
		return
	}
	id := parts[0]
	p, ok := r.get(id)
	if !ok {
		http.Error(w, "plugin not found", http.StatusNotFound)
		return
	}

	action := ""
	if len(parts) >= 2 {
		action = parts[1]
	}

	// Serve files shipped by the plugin itself (web/index.html, css, js, ...).
	if action == "web" && req.Method == http.MethodGet {
		rel := strings.Join(parts[2:], "/")
		if rel == "" {
			rel = "index.html"
		}
		prefix := "/api/plugins/" + id + "/"
		http.StripPrefix(prefix, http.FileServer(http.Dir(p.meta.Dir))).ServeHTTP(w, req)
		return
	}

	switch {
	case action == "call" && req.Method == http.MethodPost:
		var body callRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		r.ensureFirstRun(id)
		if err := p.callMethod(body.Method, body.Session, body.Args); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})

	case action == "lifecycle" && req.Method == http.MethodPost:
		var body lifecycleRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		switch body.Hook {
		case "on_ui_show":
			r.ensureFirstRun(id)
			if err := r.sessions.start(body.Session); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		case "on_close":
			r.sessions.close(body.Session)
		}
		p.callHook(body.Hook, body.Session)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})

	case action == "events" && req.Method == http.MethodGet:
		r.sessions.serveEvents(w, req, req.URL.Query().Get("session"))

	default:
		http.NotFound(w, req)
	}
}
