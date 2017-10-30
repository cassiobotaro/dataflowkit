package fetch

import (
	"fmt"
	neturl "net/url"
	"strings"

	"github.com/slotix/dataflowkit/errs"
	"github.com/slotix/dataflowkit/scrape"
	"github.com/slotix/dataflowkit/splash"
	"github.com/temoto/robotstxt"
)

// Define service interface
type Service interface {
	Fetch(req interface{}) (interface{}, error)
	getURL(req interface{}) string
	Response(req interface{}) (interface{}, error)
}

// Implement service with empty struct
type FetchService struct {
}

// create type that return function.
// this will be needed in main.go
type ServiceMiddleware func(Service) Service

func (fs FetchService) getURL(req interface{}) string {
	var url string
	switch req.(type) {
	case splash.Request:
		url = req.(splash.Request).URL
	case scrape.HttpClientFetcherRequest:
		url = req.(scrape.HttpClientFetcherRequest).URL
	}
	//trim trailing slash if any.
	//aws s3 bucket item name cannot contain slash at the end.
	return strings.TrimSpace(strings.TrimRight(url, "/"))
}

//Fetch returns splash.Response
//see transport.go encodeFetchResponse for more details about retured value.

func (fs FetchService) Fetch(req interface{}) (interface{}, error) {
	res, err := fs.Response(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

//Response returns splash.Response
//see transport.go encodeResponse for more details about retured value.
func (fs FetchService) Response(req interface{}) (interface{}, error) {
	request := req.(splash.Request)
	fetcher, err := scrape.NewSplashFetcher()
	if err != nil {
		logger.Println(err)
	}
	res, err := fetcher.Fetch(request)
	if err != nil {
		return nil, err
	}
	return res, nil
}
