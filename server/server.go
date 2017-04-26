package server

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/julienschmidt/httprouter"

	kitlog "github.com/go-kit/kit/log"
	kitprometheus "github.com/go-kit/kit/metrics/prometheus"
	httptransport "github.com/go-kit/kit/transport/http"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

var logger *log.Logger

func init() {
	logger = log.New(os.Stdout, "server: ", log.Lshortfile)
}

func Init(port string) {

	var serverLogger kitlog.Logger
	serverLogger = kitlog.NewLogfmtLogger(os.Stderr)
	serverLogger = kitlog.With(serverLogger, "caller", kitlog.DefaultCaller, "Time", time.Now().Format("Jan _2 15:04:05"))

	fieldKeys := []string{"method", "error"}

	requestCount := kitprometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Namespace: "dfk",
		Subsystem: "parse_service",
		Name:      "request_count",
		Help:      "Number of requests received.",
	}, fieldKeys)
	requestLatency := kitprometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
		Namespace: "dfk",
		Subsystem: "parse_service",
		Name:      "request_latency_microseconds",
		Help:      "Total duration of requests in microseconds.",
	}, fieldKeys)
	countResult := kitprometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
		Namespace: "dfk",
		Subsystem: "parse_service",
		Name:      "count_result",
		Help:      "The result of each count method.",
	}, []string{})

	var svc ParseService
	svc = parseService{}
	svc = statsMiddleware("18")(svc)
	svc = cachingMiddleware()(svc)
	svc = loggingMiddleware(serverLogger)(svc)
	svc = robotsTxtMiddleware()(svc)
	svc = instrumentingMiddleware(requestCount, requestLatency, countResult)(svc)

	fetchHandler := httptransport.NewServer(
		makeFetchEndpoint(svc),
		decodeFetchRequest,
		encodeResponse,
	)

	marshalDataHandler := httptransport.NewServer(
		makeMarshalDataEndpoint(svc),
		decodeMarshalDataRequest,
		encodeResponse,
	)

	/*
		checkServicesHandler := httptransport.NewServer(
			makeCheckServicesEndpoint(svc),
			decodeCheckServicesRequest,
			encodeCheckServicesResponse,
		)
	*/

	router := httprouter.New()
	router.Handler("POST", "/app/gethtml", fetchHandler)
	router.Handler("POST", "/app/marshaldata", marshalDataHandler)
	//router.Handler("POST", "/app/chkservices", checkServicesHandler)
	/*
		router.ServeFiles("/static/*filepath", http.Dir("web/static"))
		router.HandlerFunc("GET", "/", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, "web/index.html")
		})
		router.HandlerFunc("GET", "/get_started", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, "web/get_started.html")
		})
	*/
	router.Handler("GET", "/metrics", stdprometheus.Handler())

	serverLogger.Log("msg", "HTTP", "addr", port)
	serverLogger.Log("err", http.ListenAndServe(port, router))
}
