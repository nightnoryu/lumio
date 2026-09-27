package httptransport

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nightnoryu/go-kita/log"

	"lumio/api/server/publicapi"
	"lumio/internal/app"
	"lumio/internal/domain"
)

const cookieName = "__Host-lumio-session"

type APIConfig struct {
	Service    *app.Service
	Media      *app.Media
	Origin     string
	BaseDomain string
}
type requestKey struct{}
type requestState struct {
	writer http.ResponseWriter
	user   domain.User
	token  string
}
type apiHandler struct {
	statusHandler
	config APIConfig
	logger log.Logger
}

func request(ctx context.Context) requestState {
	state, _ := ctx.Value(requestKey{}).(requestState)
	return state
}
func setSession(ctx context.Context, token string) {
	cookie := &http.Cookie{Name: cookieName, Value: token, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: int(app.SessionLifetime.Seconds()), Expires: time.Now().Add(app.SessionLifetime)}
	if token == "" {
		cookie.MaxAge = -1
		cookie.Expires = time.Unix(1, 0)
	}
	http.SetCookie(request(ctx).writer, cookie)
}
func (h *apiHandler) Register(ctx context.Context, req *publicapi.Registration) error {
	token, err := h.config.Service.Register(ctx, req.Email, req.Password, req.Invitation)
	if err == nil {
		setSession(ctx, token)
	}
	return err
}
func (h *apiHandler) Login(ctx context.Context, req *publicapi.Credentials) error {
	token, err := h.config.Service.Login(ctx, req.Email, req.Password)
	if err == nil {
		setSession(ctx, token)
	}
	return err
}
func (h *apiHandler) Logout(ctx context.Context) error {
	err := h.config.Service.Logout(ctx, request(ctx).token)
	if err == nil {
		setSession(ctx, "")
	}
	return err
}
func (h *apiHandler) ResetPassword(ctx context.Context, req *publicapi.PasswordReset) error {
	err := h.config.Service.ResetPassword(ctx, req.Token, req.Password)
	if err == nil {
		setSession(ctx, "")
	}
	return err
}
func (h *apiHandler) GetMe(ctx context.Context) (*publicapi.Viewer, error) {
	state := request(ctx)
	return &publicapi.Viewer{ID: state.user.ID, Email: state.user.Email, CsrfToken: app.CSRF(state.token), BaseDomain: h.config.BaseDomain}, nil
}
func apiSite(site domain.Site) *publicapi.Site {
	return &publicapi.Site{ID: site.ID, Slug: site.Slug, CreatedAt: site.CreatedAt}
}
func (h *apiHandler) CreateSite(ctx context.Context, req *publicapi.SiteInput) (*publicapi.Site, error) {
	site, err := h.config.Service.CreateSite(ctx, request(ctx).user.ID, req.Slug)
	return apiSite(site), err
}
func (h *apiHandler) ListSites(ctx context.Context) (publicapi.Sites, error) {
	sites, err := h.config.Service.Sites(ctx, request(ctx).user.ID)
	result := make(publicapi.Sites, 0, len(sites))
	for _, site := range sites {
		result = append(result, *apiSite(site))
	}
	return result, err
}
func (h *apiHandler) GetSite(ctx context.Context, params publicapi.GetSiteParams) (*publicapi.Site, error) {
	site, err := h.config.Service.Site(ctx, request(ctx).user.ID, params.ID.String())
	return apiSite(site), err
}
func (h *apiHandler) RenameSite(ctx context.Context, req *publicapi.SiteInput, params publicapi.RenameSiteParams) (*publicapi.Site, error) {
	site, err := h.config.Service.RenameSite(ctx, request(ctx).user.ID, params.ID.String(), req.Slug)
	return apiSite(site), err
}
func (h *apiHandler) NewError(_ context.Context, err error) *publicapi.ErrorStatusCode {
	status, message := http.StatusInternalServerError, "Something went wrong. Please try again."
	switch {
	case errors.Is(err, domain.ErrMediaQuota):
		status, message = http.StatusConflict, "Photo or storage quota reached. Remove unused photos and wait for cleanup before retrying."
	case errors.Is(err, domain.ErrMediaInput):
		status, message = http.StatusBadRequest, "Invalid photo. Use JPEG, PNG or WebP within the upload size limit; watermark text must be at most 80 printable ASCII characters."
	case errors.Is(err, domain.ErrInvalid):
		status, message = http.StatusBadRequest, "Invalid input. Use a valid email, a 12–128 byte password, and a nonreserved subdomain of 1–63 letters, digits or hyphens."
	case errors.Is(err, domain.ErrUnauthorized):
		status, message = http.StatusUnauthorized, domain.ErrUnauthorized.Error()
	case errors.Is(err, domain.ErrInvitation):
		status, message = http.StatusForbidden, domain.ErrInvitation.Error()
	case errors.Is(err, domain.ErrConflict):
		status, message = http.StatusConflict, domain.ErrConflict.Error()
	case errors.Is(err, domain.ErrNotFound):
		status, message = http.StatusNotFound, domain.ErrNotFound.Error()
	default:
		h.logger.Error(err, "API request failed")
	}
	return &publicapi.ErrorStatusCode{StatusCode: status, Response: publicapi.Error{Message: message}}
}
func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(publicapi.Error{Message: message})
}
func (h *apiHandler) middleware(next *publicapi.Server) http.Handler {
	origin, _ := url.Parse(h.config.Origin)
	// Bound expensive Argon2 work and request bodies before API decoding.
	hashing := make(chan struct{}, 2)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		_, known := next.FindPath(r.Method, r.URL)
		if !known || r.URL.Path == "/api/status" {
			next.ServeHTTP(w, r)
			return
		}
		if h.config.Service == nil {
			writeError(w, http.StatusServiceUnavailable, "Identity service unavailable")
			return
		}
		if origin == nil || r.Host != origin.Host {
			writeError(w, http.StatusForbidden, "Invalid dashboard host")
			return
		}
		write := r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions
		if write && (len(r.Header.Values("Origin")) != 1 || r.Header.Get("Origin") != h.config.Origin) {
			writeError(w, http.StatusForbidden, "Invalid request origin")
			return
		}
		if strings.Contains(r.URL.Path, "/photos") && h.config.Media == nil {
			writeError(w, http.StatusServiceUnavailable, "Media service unavailable")
			return
		}
		state := requestState{writer: w}
		public := r.URL.Path == "/api/auth/login" || r.URL.Path == "/api/auth/register" || r.URL.Path == "/api/auth/reset-password"
		if !public {
			var ok bool
			state, ok = h.authorize(w, r, write)
			if !ok {
				return
			}
		} else if write {
			select {
			case hashing <- struct{}{}:
				defer func() { <-hashing }()
			default:
				w.Header().Set("Retry-After", "2")
				writeError(w, http.StatusTooManyRequests, "Please try again shortly")
				return
			}
		}
		if write && !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") && r.URL.Path != "/api/auth/logout" {
			writeError(w, http.StatusUnsupportedMediaType, "Expected application/json")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestKey{}, state)))
	})
}

func (h *apiHandler) authorize(w http.ResponseWriter, r *http.Request, write bool) (requestState, bool) {
	state := requestState{writer: w}
	cookies := r.CookiesNamed(cookieName)
	if len(cookies) != 1 {
		writeError(w, http.StatusUnauthorized, "Sign in required")
		return state, false
	}
	state.token = cookies[0].Value
	var err error
	state.user, err = h.config.Service.Authenticate(r.Context(), state.token)
	if err != nil {
		response := h.NewError(r.Context(), err)
		writeError(w, response.StatusCode, response.Response.Message)
		return state, false
	}
	if write && (len(r.Header.Values("X-CSRF-Token")) != 1 || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(app.CSRF(state.token))) != 1) {
		writeError(w, http.StatusForbidden, "Invalid CSRF token")
		return state, false
	}
	return state, true
}
