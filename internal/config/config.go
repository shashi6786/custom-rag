package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds 12-factor-style settings for query and ingest binaries.
type Config struct {
	ServiceName string

	HTTPAddr string

	OpenAIAPIKey          string
	OpenAIEmbeddingModel  string
	LLMChatProvider       string
	OpenAIChatModel       string
	GoogleAPIKey          string
	GeminiChatModel       string
	QdrantHost            string
	QdrantPort            int

	OIDCIssuerURL       string
	OIDCAudience        string
	OIDCSkipTLSVerify   bool
	DisableAuth         bool
	CacheEmbedMaxEntries int
	CacheEmbedTTL       time.Duration
	CacheRetrievalEnabled bool
	CacheResponseEnabled bool
}

// Load reads configuration from environment variables. Missing values use
// safe defaults where possible; required secrets return an error when the
// corresponding feature is enabled.
func Load(serviceName string) (Config, error) {
	port := getenvInt("PORT", 8080)
	cfg := Config{
		ServiceName:          serviceName,
		HTTPAddr:             fmt.Sprintf(":%d", port),
		OpenAIEmbeddingModel: getenv("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small"),
		LLMChatProvider:      strings.ToLower(getenv("LLM_CHAT_PROVIDER", "openai")),
		OpenAIChatModel:    getenv("OPENAI_CHAT_MODEL", "gpt-4.1-mini"),
		GeminiChatModel:    getenv("GEMINI_CHAT_MODEL", "gemini-2.5-flash"),
		QdrantHost:         getenv("QDRANT_HOST", "localhost"),
		QdrantPort:         getenvInt("QDRANT_PORT", 6334),

		OIDCIssuerURL:     strings.TrimSuffix(getenv("OIDC_ISSUER_URL", ""), "/"),
		OIDCAudience:      getenv("OIDC_AUDIENCE", ""),
		OIDCSkipTLSVerify: getenvBool("OIDC_SKIP_TLS_VERIFY", false),
		DisableAuth:       getenvBool("DISABLE_AUTH", false),

		CacheEmbedMaxEntries:  getenvInt("CACHE_EMBED_MAX_ENTRIES", 5000),
		CacheEmbedTTL:         getenvDuration("CACHE_EMBED_TTL", 10*time.Minute),
		CacheRetrievalEnabled: getenvBool("CACHE_RETRIEVAL_ENABLED", false),
		CacheResponseEnabled: getenvBool("CACHE_RESPONSE_ENABLED", false),
	}

	cfg.OpenAIAPIKey = os.Getenv("OPENAI_API_KEY")
	cfg.GoogleAPIKey = getenv("GOOGLE_API_KEY", getenv("GEMINI_API_KEY", ""))

	if cfg.ServiceName == "ingest" {
		p := getenvInt("INGEST_PORT", 8081)
		cfg.HTTPAddr = fmt.Sprintf(":%d", p)
	}

	if cfg.DisableAuth {
		return cfg, nil
	}
	if cfg.OIDCIssuerURL == "" || cfg.OIDCAudience == "" {
		return Config{}, fmt.Errorf("OIDC_ISSUER_URL and OIDC_AUDIENCE are required unless DISABLE_AUTH=true")
	}
	return cfg, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getenvInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func getenvBool(k string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(k)))
	if v == "" {
		return def
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func getenvDuration(k string, def time.Duration) time.Duration {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
