package router

import (
	"go-sub/internal/appconfig"
	"go-sub/internal/auth"
	"go-sub/internal/handler"
	"go-sub/internal/middleware"
	"net/http"

	"github.com/gorilla/mux"
)

func NewRouter() *mux.Router {
	r := mux.NewRouter()
	r.Use(middleware.CorsMiddleware)

	// API routes
	api := r.PathPrefix("/api").Subrouter()
	api.Use(middleware.ApiResponseMiddleware)
	api.Use(auth.Middleware)
	api.HandleFunc("/auth/status", handler.AuthStatusHandler).Methods("GET")
	api.HandleFunc("/auth/login", handler.LoginHandler).Methods("POST")
	api.HandleFunc("/auth/logout", handler.LogoutHandler).Methods("POST")
	api.HandleFunc("/auth/setup", handler.SetupHandler).Methods("POST")
	api.HandleFunc("/auth/change-password", handler.ChangePasswordHandler).Methods("POST")
	api.HandleFunc("/sources", handler.GetSourcesHandler).Methods("GET")
	api.HandleFunc("/sources", handler.AddSourceHandler).Methods("POST")
	api.HandleFunc("/sources", handler.DeleteSourceHandler).Methods("DELETE")
	api.HandleFunc("/sources", handler.UpdateSourceHandler).Methods("PUT")
	api.HandleFunc("/sources/refresh", handler.RefreshSourcesHandler).Methods("POST")
	api.HandleFunc("/sources/test", handler.TestSourceHandler).Methods("POST")
	api.HandleFunc("/sources/data", handler.SourceDataHandler).Methods("GET")
	api.HandleFunc("/health", handler.HealthHandler).Methods("GET")
	api.HandleFunc("/version", handler.VersionHandler).Methods("GET")
	api.HandleFunc("/geoip", handler.GeoIPHandler).Methods("GET")

	// Profile CRUD
	api.HandleFunc("/profiles", handler.GetProfilesHandler).Methods("GET")
	api.HandleFunc("/profiles", handler.CreateProfileHandler).Methods("POST")
	api.HandleFunc("/profiles", handler.UpdateProfileHandler).Methods("PUT")
	api.HandleFunc("/profiles", handler.DeleteProfileHandler).Methods("DELETE")
	api.HandleFunc("/profiles/test-script", handler.TestScriptHandler).Methods("POST")

	// Routing Profile CRUD
	api.HandleFunc("/routing", handler.RoutingListHandler).Methods("GET")
	api.HandleFunc("/routing", handler.RoutingAddHandler).Methods("POST")
	api.HandleFunc("/routing", handler.RoutingUpdateHandler).Methods("PUT")
	api.HandleFunc("/routing", handler.RoutingDeleteHandler).Methods("DELETE")
	api.HandleFunc("/routing/catalog", handler.RoutingCatalogHandler).Methods("GET")

	// Custom RuleSet CRUD
	api.HandleFunc("/rulesets", handler.RuleSetListHandler).Methods("GET")
	api.HandleFunc("/rulesets", handler.RuleSetAddHandler).Methods("POST")
	api.HandleFunc("/rulesets", handler.RuleSetUpdateHandler).Methods("PUT")
	api.HandleFunc("/rulesets", handler.RuleSetDeleteHandler).Methods("DELETE")
	api.HandleFunc("/rulesets/types", handler.RuleSetTypesHandler).Methods("GET")

	// Simulation / Preview
	api.HandleFunc("/simulate", handler.SimulateHandler).Methods("POST")
	api.HandleFunc("/preview", handler.GeneratePreviewHandler).Methods("GET")

	// Subscription output by profile ID
	r.HandleFunc("/sub/{id:[a-f0-9]+}", handler.SubHandler).Methods("GET", "OPTIONS")

	// Legacy filter endpoint
	r.HandleFunc("/filter", handler.FilterHandler).Methods("GET", "OPTIONS")

	// Static file serving for the frontend (must be last)
	staticDir := appconfig.Get().FrontendPath
	r.PathPrefix("/").Handler(http.FileServer(http.Dir(staticDir)))

	return r
}
