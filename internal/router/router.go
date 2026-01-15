package router

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"potstack/internal/models"
	"potstack/internal/resource"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Router manages dynamic routing for sandboxes
type Router struct {
	RepoRoot string

	// pathRoutes: "/pot/org/name" -> Handler
	pathRoutes map[string]http.Handler

	// Track which sandbox owns which routes
	// Key: org/name -> []string (e.g. "PATH:/pot/org/name")
	sandboxRoutes map[string][]string

	mu sync.RWMutex
}

func NewRouter(repoRoot string) *Router {
	return &Router{
		RepoRoot:      repoRoot,
		pathRoutes:    make(map[string]http.Handler),
		sandboxRoutes: make(map[string][]string),
	}
}

// ServeHTTP implements http.Handler with longest prefix matching
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Find longest matching prefix
	var bestMatch string
	var bestHandler http.Handler

	for prefix, handler := range r.pathRoutes {
		if strings.HasPrefix(req.URL.Path, prefix) {
			if len(prefix) > len(bestMatch) {
				bestMatch = prefix
				bestHandler = handler
			}
		}
	}

	if bestHandler != nil {
		bestHandler.ServeHTTP(w, req)
		return
	}

	http.NotFound(w, req)
}

// UpdateRoutes updates routes for a sandbox based on pot.yml and run.yml
func (r *Router) UpdateRoutes(org, name string, includeExe bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 1. Cleanup old routes
	r.removeRoutesInternal(org, name)

	baseDir := filepath.Join(r.RepoRoot, org, fmt.Sprintf("%s.git", name))
	potFile := filepath.Join(baseDir, "data", "faaspot", "program", "pot.yml")

	// Read Config
	potData, err := os.ReadFile(potFile)
	if err != nil {
		return nil // Config missing, nothing to register
	}

	var potCfg models.PotConfig
	if err := yaml.Unmarshal(potData, &potCfg); err != nil {
		return err
	}

	var coreHandler http.Handler

	if potCfg.Type == "static" {
		// Static file handler
		coreHandler = resource.NewStaticHandler(r.RepoRoot, org, name, potCfg.Root)
	} else if potCfg.Type == "exe" && includeExe {
		// Read runtime port
		runFile := filepath.Join(baseDir, "data", "faaspot", "run.yml")
		runData, err := os.ReadFile(runFile)
		if err != nil {
			return nil // No runtime, skip exe routes
		}

		var rc models.RunConfig
		if err := yaml.Unmarshal(runData, &rc); err != nil {
			return err
		}

		if rc.Runtime.Port == 0 {
			return nil // No port assigned
		}

		target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", rc.Runtime.Port))
		coreHandler = httputil.NewSingleHostReverseProxy(target)
	} else {
		return nil // Unknown type or exe without includeExe flag
	}

	var registeredKeys []string

	// Register three standard routes with different strip rules
	// 1. /pot/{org}/{name}/* -> strip /pot/{org}/{name}
	potPrefix := fmt.Sprintf("/pot/%s/%s", org, name)
	r.pathRoutes[potPrefix] = stripPrefixHandler(potPrefix, coreHandler)
	registeredKeys = append(registeredKeys, "PATH:"+potPrefix)

	// 2. /api/{org}/{name}/* -> strip /{org}/{name}, keep /api
	apiPrefix := fmt.Sprintf("/api/%s/%s", org, name)
	r.pathRoutes[apiPrefix] = stripOrgNameHandler(org, name, coreHandler)
	registeredKeys = append(registeredKeys, "PATH:"+apiPrefix)

	// 3. /web/{org}/{name}/* -> strip /{org}/{name}, keep /web
	webPrefix := fmt.Sprintf("/web/%s/%s", org, name)
	r.pathRoutes[webPrefix] = stripOrgNameHandler(org, name, coreHandler)
	registeredKeys = append(registeredKeys, "PATH:"+webPrefix)

	r.sandboxRoutes[fmt.Sprintf("%s/%s", org, name)] = registeredKeys
	return nil
}

// stripPrefixHandler removes the entire prefix from the path
// /pot/org/name/foo -> /foo
func stripPrefixHandler(prefix string, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		path := strings.TrimPrefix(req.URL.Path, prefix)
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		req.URL.Path = path
		req.Header.Set("X-Forwarded-Prefix", prefix)
		handler.ServeHTTP(w, req)
	})
}

// stripOrgNameHandler removes /{org}/{name} but keeps the route prefix
// /api/org/name/users -> /api/users
// /web/org/name/index.html -> /web/index.html
func stripOrgNameHandler(org, name string, handler http.Handler) http.Handler {
	orgNamePart := fmt.Sprintf("/%s/%s", org, name)
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		path := strings.Replace(req.URL.Path, orgNamePart, "", 1)
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		req.URL.Path = path
		req.Header.Set("X-Forwarded-Prefix", orgNamePart)
		handler.ServeHTTP(w, req)
	})
}

// UpdateExeRoutes updates routes for exe type (with runtime port)
func (r *Router) UpdateExeRoutes(org, name string) error {
	return r.UpdateRoutes(org, name, true)
}

// UpdateStaticRoutes updates routes for static type only
func (r *Router) UpdateStaticRoutes(org, name string) error {
	return r.UpdateRoutes(org, name, false)
}

// RemoveRoutes removes all routes for a sandbox
func (r *Router) RemoveRoutes(org, name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removeRoutesInternal(org, name)
}

func (r *Router) removeRoutesInternal(org, name string) {
	key := fmt.Sprintf("%s/%s", org, name)
	if keys, ok := r.sandboxRoutes[key]; ok {
		for _, k := range keys {
			if strings.HasPrefix(k, "PATH:") {
				delete(r.pathRoutes, strings.TrimPrefix(k, "PATH:"))
			}
		}
		delete(r.sandboxRoutes, key)
	}
}
