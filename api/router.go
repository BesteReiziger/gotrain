package api

import (
	"encoding/json"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rijdendetreinen/gotrain/stores"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

var (
	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "gotrain",
		Subsystem: "api",
		Name:      "duration",
		Help:      "Duration of HTTP API requests.",
	}, []string{"path"})
)

var (
	httpReqs = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "gotrain",
		Subsystem: "api",
		Name:      "requests",
		Help:      "HTTP API requests",
	}, []string{"path"})
)

// prometheusMiddleware implements mux.MiddlewareFunc.
func prometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := mux.CurrentRoute(r)
		path, _ := route.GetPathTemplate()
		timer := prometheus.NewTimer(httpDuration.WithLabelValues(path))
		next.ServeHTTP(w, r)
		timer.ObserveDuration()
		httpReqs.WithLabelValues(path).Add(1)
	})
}

// ServeAPI serves the REST API on the given address
func ServeAPI(address string, exit chan bool) {
	srv := &http.Server{Addr: address}
	router := mux.NewRouter()

	// Router paths:
	router.HandleFunc("/version", apiVersion).Methods("GET")
	router.HandleFunc("/v1", apiVersion).Methods("GET")
	router.HandleFunc("/v2", apiVersion).Methods("GET")
	router.HandleFunc("/v2/version", apiVersion).Methods("GET")
	router.HandleFunc("/v2/status", apiStatus).Methods("GET")

	router.HandleFunc("/v2/arrivals/stats", arrivalCounters).Methods("GET")
	router.HandleFunc("/v2/arrivals/station/{station}", arrivalsStation).Methods("GET")
	router.HandleFunc("/v2/arrivals/arrival/{id}/{station}/{date}", arrivalDetails).Methods("GET")

	router.HandleFunc("/v2/departures/stats", departureCounters).Methods("GET")
	router.HandleFunc("/v2/departures/station/{station}", departuresStation).Methods("GET")
	router.HandleFunc("/v2/departures/uic/{station}", departuresUicStation).Methods("GET")
	router.HandleFunc("/v2/departures/departure/{id}/{station}/{date}", departureDetails).Methods("GET")

	router.HandleFunc("/v2/services/stats", serviceCounters).Methods("GET")
	router.HandleFunc("/v2/services/service/{id}/{date}", serviceDetails).Methods("GET")

	router.Use(prometheusMiddleware)
	srv.Handler = router

	go listenAndServe(srv, exit)
	log.Info().Str("address", address).Msg("REST API started")

	<-exit
	log.Info().Msg("Shutting down REST API")
	srv.Close()
	log.Info().Msg("REST API shut down")
	exit <- true
}

func listenAndServe(srv *http.Server, exit chan bool) {
	if err := srv.ListenAndServe(); err != nil {
		if err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("REST API fatal error")
		}
	}
}

func apiVersion(w http.ResponseWriter, r *http.Request) {
	version := map[string]int{
		"version": 2,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(version)
}

func apiStatus(w http.ResponseWriter, r *http.Request) {
	version := map[string]string{
		"arrivals":   stores.Stores.ArrivalStore.Status,
		"departures": stores.Stores.DepartureStore.Status,
		"services":   stores.Stores.ServiceStore.Status,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(version)
}
