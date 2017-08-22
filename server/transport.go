package server

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-kit/kit/log"

	"context"

	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	"github.com/slotix/dataflowkit/splash"
)

var (
	// ErrBadRouting is returned when an expected path variable is missing.
	ErrBadRouting = errors.New("inconsistent mapping between route and handler (programmer error)")
	//ErrInvalidURL is returned if validation of URL fails
	ErrInvalidURL = errors.New("invalid URL specified")
)

//decodeFetchRequest
func decodeFetchRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var request splash.Request
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		logger.Println(err)
		return nil, err
	}
	//logger.Println(request)
	//request.URL normalization and validation
	reqURL := strings.TrimSpace(request.URL)
	if _, err := url.ParseRequestURI(reqURL); err != nil {
		return nil, err
	}
	request.URL = reqURL
	logger.Println("transport request",request.URL)
	return request, nil
}

func encodeFetchResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	if e, ok := response.(errorer); ok && e.error() != nil {
		// Not a Go kit transport error, but a business-logic error.
		// Provide those as HTTP errors.
		encodeError(ctx, e.error(), w)
		return nil
	}
	sResponse := response.(*splash.Response)
	logger.Println("transport response", sResponse.Error)
	if sResponse.Error != ""{
		return errors.New(sResponse.Error)
	}
	content, err := sResponse.GetContent()
	if err != nil {
		return err
	}
	data, err := ioutil.ReadAll(content)

	if err != nil {
		return err
	}
	_, err = w.Write(data)

	if err != nil {
		return err
	}
	return nil
}

//decodeParseRequest
func decodeParseRequest(_ context.Context, r *http.Request) (interface{}, error) {
	request, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	return request, nil
}

func encodeParseResponse(_ context.Context, w http.ResponseWriter, response interface{}) error {
	data, err := ioutil.ReadAll(response.(io.Reader))
	if err != nil {
		return err
	}
	_, err = w.Write(data)

	if err != nil {
		return err
	}
	return nil
}

// errorer is implemented by all concrete response types that may contain
// errors. It allows us to change the HTTP response code without needing to
// trigger an endpoint (transport-level) error.
type errorer interface {
	error() error
}

// encode error
func encodeError(_ context.Context, err error, w http.ResponseWriter) {
	if err == nil {
		panic("encodeError with nil error")
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": err.Error(),
	})
}

// Make Http Handler
func MakeHttpHandler(ctx context.Context, endpoint Endpoints, logger log.Logger) http.Handler {
	/*
		router := httprouter.New()
		var svc Service
		fetchHandler := httptransport.NewServer(
			MakeFetchEndpoint(svc),
			decodeFetchRequest,
			encodeFetchResponse,
		)
		router.Handler("POST", "/app/fetch", fetchHandler)
		return router
	*/
	r := mux.NewRouter()
	options := []httptransport.ServerOption{
		httptransport.ServerErrorLogger(logger),
		httptransport.ServerErrorEncoder(encodeError),
	}

	r.Methods("POST").Path("/app/fetch").Handler(httptransport.NewServer(
		endpoint.FetchEndpoint,
		decodeFetchRequest,
		encodeFetchResponse,
		options...,
	))

	r.Methods("POST").Path("/app/parse").Handler(httptransport.NewServer(
		endpoint.ParseEndpoint,
		decodeParseRequest,
		encodeParseResponse,
		options...,
	))

	return r
}
