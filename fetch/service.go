package fetch

import (
	"github.com/slotix/dataflowkit/splash"
)

// Service defines Fetch service interface
type Service interface {
	Fetch(req FetchRequester) (FetchResponser, error)
	Response(req FetchRequester) (FetchResponser, error)
}

// FetchService implements service with empty struct
type FetchService struct {
}

// ServiceMiddleware defines a middleware for a Fetch service
type ServiceMiddleware func(Service) Service

//Fetch downloads web page content and returns it
//See EncodeSplashFetcherContent and EncodeBaseFetcherContent methods implementation in transport.go for more details.
func (fs FetchService) Fetch(req FetchRequester) (FetchResponser, error) {
	res, err := fs.Response(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

//Response returns splash.Response
//See EncodeSplashFetcherResponse and EncodeBaseFetcherResponse methods implementation in transport.go for more details.
func (fs FetchService) Response(req FetchRequester) (FetchResponser, error) {

	var err error
	var fetcher Fetcher
	switch req.(type) {
	case BaseFetcherRequest:
		fetcher, err = NewFetcher(Base)
	case splash.Request:
		fetcher, err = NewFetcher(Splash)
	default:
		panic("invalid fetcher request")
	}

	if err != nil {
		logger.Error(err)
	}
	res, err := fetcher.Fetch(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}
