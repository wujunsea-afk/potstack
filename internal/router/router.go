package router

import (
	"fmt"
	"log"
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

	log.Printf("[Router] ServeHTTP: path=%s, registered routes count=%d", req.URL.Path, len(r.pathRoutes))

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
		log.Printf("[Router] Matched prefix: %s", bestMatch)
		bestHandler.ServeHTTP(w, req)
		return
	}

	log.Printf("[Router] No route matched for path: %s", req.URL.Path)
	http.NotFound(w, req)
}

// RegisterStatic 注册 static 类型路由（直接从 Git 服务文件）
func (r *Router) RegisterStatic(org, name string, potCfg *models.PotConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 1. 清理旧路由
	r.removeRoutesInternal(org, name)

	// 2. 创建 Static Handler
	handler := resource.NewStaticHandler(r.RepoRoot, org, name, potCfg.Root)

	// 3. 注册三个路由
	r.registerThreeRoutesInternal(org, name, handler)
	return nil
}

// RegisterExe 注册 exe 类型路由（需要读取 run.yml）
func (r *Router) RegisterExe(org, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 1. 清理旧路由
	r.removeRoutesInternal(org, name)

	// 2. 读取 run.yml 获取端口
	baseDir := filepath.Join(r.RepoRoot, org, fmt.Sprintf("%s.git", name))
	runFile := filepath.Join(baseDir, "data", "faaspot", "run.yml")

	runData, err := os.ReadFile(runFile)
	if err != nil {
		return fmt.Errorf("run.yml not found: %w", err)
	}

	var rc models.RunConfig
	if err := yaml.Unmarshal(runData, &rc); err != nil {
		return err
	}

	if rc.Runtime.Port == 0 {
		return fmt.Errorf("no port assigned")
	}

	// 3. 创建 Reverse Proxy Handler
	target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", rc.Runtime.Port))
	handler := httputil.NewSingleHostReverseProxy(target)

	// 4. 注册三个路由
	r.registerThreeRoutesInternal(org, name, handler)
	return nil
}

// registerThreeRoutesInternal 注册 /pot、/api、/web、/admin 四个前缀路由
func (r *Router) registerThreeRoutesInternal(org, name string, handler http.Handler) {
	var registeredKeys []string

	// 1. /pot/{org}/{name}/* -> 去掉 /pot/{org}/{name}
	potPrefix := fmt.Sprintf("/pot/%s/%s", org, name)
	r.pathRoutes[potPrefix] = stripPrefixHandler(potPrefix, handler)
	registeredKeys = append(registeredKeys, "PATH:"+potPrefix)
	log.Printf("[Router] Registered route: %s", potPrefix)

	// 2. /api/{org}/{name}/* -> 去掉 /{org}/{name}
	apiPrefix := fmt.Sprintf("/api/%s/%s", org, name)
	r.pathRoutes[apiPrefix] = stripOrgNameHandler(org, name, handler)
	registeredKeys = append(registeredKeys, "PATH:"+apiPrefix)
	log.Printf("[Router] Registered route: %s", apiPrefix)

	// 3. /web/{org}/{name}/* -> 去掉 /{org}/{name}
	webPrefix := fmt.Sprintf("/web/%s/%s", org, name)
	r.pathRoutes[webPrefix] = stripOrgNameHandler(org, name, handler)
	registeredKeys = append(registeredKeys, "PATH:"+webPrefix)
	log.Printf("[Router] Registered route: %s", webPrefix)

	// 4. /admin/{org}/{name}/* -> 去掉 /{org}/{name}
	adminPrefix := fmt.Sprintf("/admin/%s/%s", org, name)
	r.pathRoutes[adminPrefix] = stripOrgNameHandler(org, name, handler)
	registeredKeys = append(registeredKeys, "PATH:"+adminPrefix)
	log.Printf("[Router] Registered route: %s", adminPrefix)

	r.sandboxRoutes[fmt.Sprintf("%s/%s", org, name)] = registeredKeys
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
