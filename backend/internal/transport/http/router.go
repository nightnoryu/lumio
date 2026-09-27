package httptransport

import (
	"context"
	"io/fs"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/nightnoryu/go-kita/health"
	"github.com/nightnoryu/go-kita/log"

	"lumio/api/server/publicapi"
)

func NewRouter(assets fs.FS, ping health.Check, logger log.Logger, configs ...APIConfig) (http.Handler, error) {
	config := APIConfig{}
	if len(configs) > 0 {
		config = configs[0]
	}
	handler := &apiHandler{config: config, logger: logger}
	api, err := publicapi.NewServer(handler)
	if err != nil {
		return nil, err
	}
	live, err := health.NewLivenessHandler(health.LivenessConfig{})
	if err != nil {
		return nil, err
	}
	ready, err := health.NewReadinessHandler(health.ReadinessConfig{
		Checks:    []health.NamedCheck{{Name: "postgresql", Check: ping}},
		Timeout:   3 * time.Second,
		OnFailure: func(name string, err error) { logger.Error(err, name+" readiness check failed") },
	})
	if err != nil {
		return nil, err
	}
	router := mux.NewRouter()
	router.Handle("/livez", live).Methods(http.MethodGet)
	router.Handle("/healthz", ready).Methods(http.MethodGet)
	router.HandleFunc("/portfolio.js", viewerScript).Methods(http.MethodGet, http.MethodHead)
	router.HandleFunc("/preview/{id}", handler.previewPortfolio).Methods(http.MethodGet)
	router.PathPrefix("/api/").Handler(handler.middleware(api))
	router.Handle("/api", http.NotFoundHandler())
	router.PathPrefix("/assets/").Handler(http.FileServer(http.FS(assets))).Methods(http.MethodGet, http.MethodHead)
	router.Handle("/", http.FileServer(http.FS(assets))).Methods(http.MethodGet, http.MethodHead)
	return handler.hostRouter(router), nil
}

type statusHandler struct{}

func (statusHandler) GetStatus(context.Context) (*publicapi.GetStatusOK, error) {
	return &publicapi.GetStatusOK{Status: publicapi.GetStatusOKStatusOk}, nil
}
