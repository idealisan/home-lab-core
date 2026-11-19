package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
)

type windowConfig struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type pluginMeta struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Version     string       `json:"version"`
	Description string       `json:"description"`
	Icon        string       `json:"icon"`
	Menu        string       `json:"menu"`
	Web         string       `json:"web"`
	Dock        bool         `json:"dock"`
	Window      windowConfig `json:"window"`
	Dir         string       `json:"-"`
}

type plugin struct {
	meta         pluginMeta
	L            *lua.LState
	mu           sync.Mutex
	firstRunDone bool
}

type pluginRegistry struct {
	mu         sync.RWMutex
	plugins    map[string]*plugin
	order      []string
	sessions   *sessionManager
	config     *configStore
	installDir string
}

type configStore struct {
	mu   sync.RWMutex
	data map[string]string
}

func newConfigStore() *configStore {
	return &configStore{data: map[string]string{}}
}

func newPluginRegistry(sm *sessionManager) (*pluginRegistry, error) {
	r := &pluginRegistry{
		plugins:    map[string]*plugin{},
		sessions:   sm,
		config:     newConfigStore(),
		installDir: filepath.Join("tmp", "plugin-state"),
	}
	dir := pluginsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("no plugins directory at %s", dir)
			return r, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pluginDir := filepath.Join(dir, e.Name())
		path := filepath.Join(pluginDir, "plugin.lua")
		if _, err := os.Stat(path); err != nil {
			continue
		}
		p, err := r.load(path, pluginDir)
		if err != nil {
			log.Printf("plugin %s: %v", e.Name(), err)
			continue
		}
		r.plugins[p.meta.ID] = p
		r.order = append(r.order, p.meta.ID)
		log.Printf("plugin loaded: %s (%s)", p.meta.ID, p.meta.Name)
	}
	return r, nil
}

func (r *pluginRegistry) load(path, dir string) (*plugin, error) {
	L := lua.NewState()
	p := &plugin{meta: pluginMeta{Dir: dir}, L: L}
	r.registerHostAPI(p)

	if err := L.DoFile(path); err != nil {
		L.Close()
		return nil, err
	}
	tbl, ok := L.GetGlobal("plugin").(*lua.LTable)
	if !ok {
		L.Close()
		return nil, fmt.Errorf("missing plugin table")
	}
	p.meta.ID = stringField(tbl, "id")
	p.meta.Name = stringField(tbl, "name")
	p.meta.Version = stringField(tbl, "version")
	p.meta.Description = stringField(tbl, "description")
	p.meta.Icon = stringField(tbl, "icon")
	p.meta.Menu = stringField(tbl, "menu")
	p.meta.Web = stringField(tbl, "web")
	p.meta.Dock = boolField(tbl, "dock")
	if w, ok := tbl.RawGetString("window").(*lua.LTable); ok {
		p.meta.Window.Width = intField(w, "width")
		p.meta.Window.Height = intField(w, "height")
	}
	if p.meta.ID == "" {
		L.Close()
		return nil, fmt.Errorf("plugin table missing id")
	}
	if p.meta.Name == "" {
		p.meta.Name = p.meta.ID
	}
	if p.meta.Web == "" {
		p.meta.Web = "web/index.html"
	}
	if p.meta.Window.Width <= 0 {
		p.meta.Window.Width = 720
	}
	if p.meta.Window.Height <= 0 {
		p.meta.Window.Height = 460
	}

	// on_load runs on every load into a Lua VM.
	p.callHook("on_load", "")
	// on_install runs once, the first time this plugin is ever seen.
	marker := filepath.Join(r.installDir, p.meta.ID+".installed")
	if _, err := os.Stat(marker); os.IsNotExist(err) {
		p.callHook("on_install", "")
		if mkErr := os.MkdirAll(r.installDir, 0o755); mkErr == nil {
			_ = os.WriteFile(marker, []byte(time.Now().Format(time.RFC3339)), 0o644)
		}
	}
	return p, nil
}

func (r *pluginRegistry) registerHostAPI(p *plugin) {
	L := p.L
	host := L.NewTable()

	L.SetField(host, "log", L.NewFunction(func(L *lua.LState) int {
		level := L.OptString(1, "info")
		msg := L.OptString(2, "")
		log.Printf("[plugin:%s/%s] %s", p.meta.ID, level, msg)
		return 0
	}))

	L.SetField(host, "exec", L.NewFunction(func(L *lua.LState) int {
		result, err := runCommand(L.OptString(1, ""))
		tbl := L.NewTable()
		L.SetField(tbl, "stdout", lua.LString(result.stdout))
		L.SetField(tbl, "stderr", lua.LString(result.stderr))
		L.SetField(tbl, "code", lua.LNumber(result.code))
		if err != nil {
			L.SetField(tbl, "error", lua.LString(err.Error()))
		}
		L.Push(tbl)
		return 1
	}))

	L.SetField(host, "emit", L.NewFunction(func(L *lua.LState) int {
		r.sessions.emit(L.OptString(1, ""), L.OptString(2, ""))
		return 0
	}))

	L.SetField(host, "shell_start", L.NewFunction(func(L *lua.LState) int {
		if err := r.sessions.start(L.OptString(1, "")); err != nil {
			L.Push(lua.LString(err.Error()))
			return 1
		}
		L.Push(lua.LNil)
		return 1
	}))

	L.SetField(host, "shell_write", L.NewFunction(func(L *lua.LState) int {
		if err := r.sessions.write(L.OptString(1, ""), L.OptString(2, "")); err != nil {
			L.Push(lua.LString(err.Error()))
			return 1
		}
		L.Push(lua.LNil)
		return 1
	}))

	L.SetField(host, "shell_close", L.NewFunction(func(L *lua.LState) int {
		r.sessions.close(L.OptString(1, ""))
		return 0
	}))

	L.SetField(host, "get_config", L.NewFunction(func(L *lua.LState) int {
		r.config.mu.RLock()
		v := r.config.data[p.meta.ID+":"+L.OptString(1, "")]
		r.config.mu.RUnlock()
		L.Push(lua.LString(v))
		return 1
	}))

	L.SetField(host, "set_config", L.NewFunction(func(L *lua.LState) int {
		r.config.mu.Lock()
		r.config.data[p.meta.ID+":"+L.OptString(1, "")] = L.OptString(2, "")
		r.config.mu.Unlock()
		return 0
	}))

	L.SetGlobal("host", host)
}

func (p *plugin) callHook(name, sessionID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	fn := p.L.GetGlobal(name)
	if fn == lua.LNil {
		return
	}
	var args []lua.LValue
	if sessionID != "" {
		args = append(args, p.sessionValue(sessionID))
	}
	if err := p.L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}, args...); err != nil {
		log.Printf("plugin %s %s: %v", p.meta.ID, name, err)
	}
}

func (p *plugin) callMethod(method, sessionID string, args []string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	fn := p.L.GetGlobal(method)
	if fn == lua.LNil {
		return fmt.Errorf("method %s not found", method)
	}
	vals := []lua.LValue{p.sessionValue(sessionID)}
	for _, a := range args {
		vals = append(vals, lua.LString(a))
	}
	return p.L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}, vals...)
}

func (p *plugin) sessionValue(id string) *lua.LTable {
	t := p.L.NewTable()
	p.L.SetField(t, "id", lua.LString(id))
	return t
}

func (r *pluginRegistry) ensureFirstRun(id string) {
	r.mu.Lock()
	p, ok := r.plugins[id]
	if !ok || p.firstRunDone {
		r.mu.Unlock()
		return
	}
	p.firstRunDone = true
	r.mu.Unlock()
	p.callHook("on_first_run", "")
}

func (r *pluginRegistry) list() []pluginMeta {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]pluginMeta, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.plugins[id].meta)
	}
	return out
}

func (r *pluginRegistry) get(id string) (*plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[id]
	return p, ok
}

type commandResult struct {
	stdout string
	stderr string
	code   int
}

func runCommand(cmd string) (commandResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var name string
	var args []string
	if runtime.GOOS == "windows" {
		name, args = "cmd.exe", []string{"/C", cmd}
	} else {
		name, args = "/bin/sh", []string{"-c", cmd}
	}
	c := exec.CommandContext(ctx, name, args...)
	var out, errOut bytes.Buffer
	c.Stdout = &out
	c.Stderr = &errOut
	err := c.Run()
	result := commandResult{stdout: out.String(), stderr: errOut.String()}
	if exitErr, ok := err.(*exec.ExitError); ok {
		result.code = exitErr.ExitCode()
	} else if err != nil {
		return result, err
	}
	return result, nil
}

func pluginsDir() string {
	if v := os.Getenv("HOME_LAB_PLUGINS_DIR"); v != "" {
		return v
	}
	if _, err := os.Stat("plugins"); err == nil {
		return "plugins"
	}
	exe, err := os.Executable()
	if err != nil {
		return "plugins"
	}
	return filepath.Join(filepath.Dir(exe), "plugins")
}

func stringField(t *lua.LTable, key string) string {
	v := t.RawGetString(key)
	if v == lua.LNil {
		return ""
	}
	return v.String()
}

func intField(t *lua.LTable, key string) int {
	v, ok := t.RawGetString(key).(lua.LNumber)
	if !ok {
		return 0
	}
	return int(v)
}

func boolField(t *lua.LTable, key string) bool {
	v, ok := t.RawGetString(key).(lua.LBool)
	return ok && bool(v)
}
