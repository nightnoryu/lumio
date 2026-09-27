package httptransport

import (
	"net/http"
	"strings"

	"github.com/felixge/httpsnoop"
	"github.com/nightnoryu/go-kita/log"

	"lumio/api/server/publicapi"
)

func (h *apiHandler) observe(next http.Handler, api *publicapi.Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := "not_found"
		switch {
		case r.URL.Path == "/":
			route = "page"
		case r.URL.Path == healthPath || r.URL.Path == livePath:
			route = "health"
		case strings.HasPrefix(r.URL.Path, "/images/"):
			route = "image"
		case strings.HasPrefix(r.URL.Path, "/preview/"):
			route = "preview"
		case strings.HasPrefix(r.URL.Path, "/assets/") || r.URL.Path == viewerPath || r.URL.Path == recoveryPath:
			route = "asset"
		default:
			if match, ok := api.FindPath(r.Method, r.URL); ok {
				route = match.Name()
			}
		}
		result := httpsnoop.CaptureMetrics(next, w, r)
		if h.config.Metrics != nil {
			h.config.Metrics.Request(route, result.Code, result.Duration)
		}
		if route != "health" {
			h.logger.WithFields(log.Fields{"route": route, "status": result.Code, "duration_ms": result.Duration.Milliseconds(), "bytes": result.Written}).Info("HTTP request")
		}
	})
}
