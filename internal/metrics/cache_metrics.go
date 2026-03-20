// Package metrics provides formatting helpers for CLASP cache metrics.
package metrics

import (
	"fmt"
	"io"

	"github.com/jedarden/clasp/internal/cache"
)

// WritePromptCachePrometheus writes prompt cache metrics in Prometheus format.
func WritePromptCachePrometheus(w io.Writer, provider string, stats cache.PromptCacheStats) {
	fmt.Fprintf(w, "# HELP clasp_prompt_cache_hits Total prompt cache hits\n")
	fmt.Fprintf(w, "# TYPE clasp_prompt_cache_hits counter\n")
	fmt.Fprintf(w, "clasp_prompt_cache_hits{provider=\"%s\"} %d\n", provider, stats.Hits)

	fmt.Fprintf(w, "# HELP clasp_prompt_cache_misses Total prompt cache misses\n")
	fmt.Fprintf(w, "# TYPE clasp_prompt_cache_misses counter\n")
	fmt.Fprintf(w, "clasp_prompt_cache_misses{provider=\"%s\"} %d\n", provider, stats.Misses)

	fmt.Fprintf(w, "# HELP clasp_prompt_cache_savings_tokens Estimated tokens saved by prompt cache\n")
	fmt.Fprintf(w, "# TYPE clasp_prompt_cache_savings_tokens counter\n")
	fmt.Fprintf(w, "clasp_prompt_cache_savings_tokens{provider=\"%s\"} %d\n", provider, stats.SavingsTokens)

	fmt.Fprintf(w, "# HELP clasp_prompt_cache_size Current number of entries in prompt cache\n")
	fmt.Fprintf(w, "# TYPE clasp_prompt_cache_size gauge\n")
	fmt.Fprintf(w, "clasp_prompt_cache_size{provider=\"%s\"} %d\n", provider, stats.Size)

	fmt.Fprintf(w, "# HELP clasp_prompt_cache_max_size Maximum prompt cache size\n")
	fmt.Fprintf(w, "# TYPE clasp_prompt_cache_max_size gauge\n")
	fmt.Fprintf(w, "clasp_prompt_cache_max_size{provider=\"%s\"} %d\n", provider, stats.MaxSize)
}
