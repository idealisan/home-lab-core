package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sync"
)

type shellSession struct {
	id    string
	cmd   *exec.Cmd
	stdin io.WriteCloser
	mu    sync.Mutex
	buf   bytes.Buffer
	subs  map[chan string]struct{}
}

func (s *shellSession) append(chunk string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf.WriteString(chunk)
	for ch := range s.subs {
		select {
		case ch <- chunk:
		default: // drop for slow clients
		}
	}
}

func (s *shellSession) snapshot() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

type sessionManager struct {
	mu       sync.Mutex
	sessions map[string]*shellSession
}

func newSessionManager() *sessionManager {
	return &sessionManager{sessions: map[string]*shellSession{}}
}

func (m *sessionManager) start(id string) error {
	if id == "" {
		return fmt.Errorf("missing session id")
	}
	m.mu.Lock()
	if s, ok := m.sessions[id]; ok && s.cmd != nil && s.cmd.Process != nil {
		m.mu.Unlock()
		return nil
	}
	s := &shellSession{id: id, subs: map[chan string]struct{}{}}
	m.sessions[id] = s
	m.mu.Unlock()

	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.Command("cmd.exe", "/Q")
	} else {
		c = exec.Command("/bin/sh")
	}
	c.Env = os.Environ()

	stdin, err := c.StdinPipe()
	if err != nil {
		m.remove(id)
		return err
	}
	stdout, err := c.StdoutPipe()
	if err != nil {
		m.remove(id)
		return err
	}
	stderr, err := c.StderrPipe()
	if err != nil {
		m.remove(id)
		return err
	}
	s.cmd = c
	s.stdin = stdin

	if err := c.Start(); err != nil {
		m.remove(id)
		return err
	}
	if runtime.GOOS == "windows" {
		_, _ = io.WriteString(stdin, "@echo off\r\n")
	}
	go copyOutput(s, stdout)
	go copyOutput(s, stderr)
	go func() {
		_ = c.Wait()
		m.remove(id)
	}()
	return nil
}

func copyOutput(s *shellSession, r io.Reader) {
	br := bufio.NewReader(r)
	buf := make([]byte, 4096)
	for {
		n, err := br.Read(buf)
		if n > 0 {
			s.append(string(buf[:n]))
		}
		if err != nil {
			return
		}
	}
}

func (m *sessionManager) write(id, input string) error {
	m.mu.Lock()
	s := m.sessions[id]
	m.mu.Unlock()
	if s == nil || s.stdin == nil {
		return fmt.Errorf("session not started: %s", id)
	}
	_, err := io.WriteString(s.stdin, input)
	return err
}

func (m *sessionManager) emit(id, chunk string) {
	m.mu.Lock()
	s := m.sessions[id]
	m.mu.Unlock()
	if s != nil {
		s.append(chunk)
	}
}

func (m *sessionManager) close(id string) {
	m.mu.Lock()
	s := m.sessions[id]
	delete(m.sessions, id)
	m.mu.Unlock()
	if s == nil {
		return
	}
	if s.stdin != nil {
		_ = s.stdin.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
}

func (m *sessionManager) remove(id string) {
	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()
}

func (m *sessionManager) subscribe(id string) chan string {
	ch := make(chan string, 128)
	m.mu.Lock()
	if s := m.sessions[id]; s != nil {
		s.mu.Lock()
		s.subs[ch] = struct{}{}
		s.mu.Unlock()
	}
	m.mu.Unlock()
	return ch
}

func (m *sessionManager) unsubscribe(id string, ch chan string) {
	m.mu.Lock()
	if s := m.sessions[id]; s != nil {
		s.mu.Lock()
		delete(s.subs, ch)
		s.mu.Unlock()
	}
	m.mu.Unlock()
}

func (m *sessionManager) snapshot(id string) string {
	m.mu.Lock()
	s := m.sessions[id]
	m.mu.Unlock()
	if s == nil {
		return ""
	}
	return s.snapshot()
}

// serveEvents streams shell output to the plugin iframe via Server-Sent Events.
func (m *sessionManager) serveEvents(w http.ResponseWriter, r *http.Request, id string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := m.subscribe(id)
	defer m.unsubscribe(id, ch)

	writeEvent := func(text string) {
		payload, _ := json.Marshal(map[string]string{"chunk": text})
		_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
		flusher.Flush()
	}
	writeEvent(m.snapshot(id))

	for {
		select {
		case chunk := <-ch:
			writeEvent(chunk)
		case <-r.Context().Done():
			return
		}
	}
}
