// oauth-dev is a tiny local HTTP server for Phase-1 OAuth redirects from Keycloak.
//
// Run: go run ./cmd/oauth-dev
//
// Default listen address: 127.0.0.1:5555 (loopback only). Override with OAUTH_DEV_ADDR
// (e.g. ":5555" for all interfaces — not recommended on untrusted networks).
//
// Register this exact redirect URI in Keycloak client "rag-query" (or your dev client):
//
//	http://127.0.0.1:5555/oauth-callback
//
// Optional second URI if you use "localhost" in the browser bar:
//
//	http://localhost:5555/oauth-callback
package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const callbackPath = "/oauth-callback"

func main() {
	addr := getenv("OAUTH_DEV_ADDR", "127.0.0.1:5555")
	if !strings.Contains(addr, ":") {
		log.Fatalf("OAUTH_DEV_ADDR must include a port, e.g. 127.0.0.1:5555, got %q", addr)
	}

	http.HandleFunc(callbackPath, callbackHandler)
	http.HandleFunc("/", rootHandler(addr))

	log.Printf("oauth-dev listening on http://%s", addr)
	log.Printf("register in Keycloak → Valid redirect URIs: http://127.0.0.1:%s%s", hostPort(addr), callbackPath)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func hostPort(addr string) string {
	// "127.0.0.1:5555" -> "5555"; ":5555" -> "5555"
	i := strings.LastIndex(addr, ":")
	if i < 0 {
		return addr
	}
	return addr[i+1:]
}

func rootHandler(listenAddr string) http.HandlerFunc {
	redirect127 := fmt.Sprintf("http://127.0.0.1:%s%s", hostPort(listenAddr), callbackPath)
	redirectLocal := fmt.Sprintf("http://localhost:%s%s", hostPort(listenAddr), callbackPath)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, `<!DOCTYPE html><html><head><meta charset="utf-8"><title>oauth-dev</title></head><body>
<h1>oauth-dev</h1>
<p>Use this tool with Keycloak Authorization Code flow. Start <code>go run ./cmd/oauth-dev</code>, then set <strong>Valid redirect URIs</strong> on client <code>rag-query</code> to <strong>exactly</strong>:</p>
<ul>
  <li><code>%s</code> (recommended)</li>
  <li><code>%s</code> (optional, if your browser uses localhost)</li>
</ul>
<p>Open your authorize URL with <code>redirect_uri</code> URL-encoded to match one of the above.</p>
<p>After login, Keycloak redirects here → the handler prints query parameters to the <strong>terminal</strong> and shows them in the browser.</p>
</body></html>`, redirect127, redirectLocal)
	}
}

func callbackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.RawQuery
	log.Printf("oauth-dev %s: %s", callbackPath, q)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!DOCTYPE html><html><head><meta charset="utf-8"><title>oauth callback</title></head><body>
<h1>OAuth redirect received</h1>
<p>Copy <code>code</code> (and <code>state</code>) into your token-exchange CLI. This was also printed to the terminal where <code>oauth-dev</code> is running.</p>
<pre style="background:#f4f4f4;padding:1em;">%s</pre>
</body></html>`, htmlEsc(q))

	if q != "" {
		if vals, err := url.ParseQuery(q); err == nil {
			if code := vals.Get("code"); code != "" {
				log.Printf("authorization code: %s", code)
			}
			if st := vals.Get("state"); st != "" {
				log.Printf("state: %s", st)
			}
			if errParam := vals.Get("error"); errParam != "" {
				log.Printf("error from IdP: %s — %s", errParam, vals.Get("error_description"))
			}
		}
	}
}

func htmlEsc(s string) string {
	replacer := strings.NewReplacer(
		`&`, "&amp;",
		`<`, "&lt;",
		`>`, "&gt;",
		`"`, "&quot;",
	)
	return replacer.Replace(s)
}

func getenv(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}
