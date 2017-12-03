package fetch

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pquerna/cachecontrol"
	"github.com/pquerna/cachecontrol/cacheobject"
	"github.com/slotix/dataflowkit/errs"
)

//BaseFetcherRequest struct collects requests information used by BaseFetcher
type BaseFetcherRequest struct {
	URL    string //URL to be retrieved
	Method string //HTTP method : GET, POST
}

//BaseFetcherResponse struct groups Response data together after retrieving it by BaseFetcher
type BaseFetcherResponse struct {
	//Response is used for determining Cacheable and Expires values. It should be omited when marshaling to intermediary cache.
	Response          *http.Response `json:"-"`
	HTML              []byte         `json:"html"`
	ReasonsNotToCache []cacheobject.Reason
	//Cacheable checks if html page is cacheable. If no then it will be downloaded every time it is requested.
	//Cacheable  bool
	//Expires - How long object stay in a cache before Splash fetcher forwards another request to an origin.
	Expires    time.Time
	StatusCode int
	Status     string
}

//MarshalJSON customizes marshaling of http.Response.Body which has type io.ReadCloser. It cannot be marshaled with standard Marshal method without casting to []byte.
//http://choly.ca/post/go-json-marshalling/
func (r *BaseFetcherResponse) MarshalJSON() ([]byte, error) {
	type Alias BaseFetcherResponse
	body, err := ioutil.ReadAll(r.Response.Body)
	if err != nil {
		return nil, err
	}
	return json.Marshal(&struct {
		HTML []byte `json:"-"`
		*Alias
	}{
		HTML:  body,
		Alias: (*Alias)(r),
	})
}

//setCacheInfo check if resource is cacheable
//r.Cacheable and r.CacheExpirationTime fields are filled inside this func
func (r *BaseFetcherResponse) SetCacheInfo() {
	reasons, expires, err := cachecontrol.CachableResponse(r.Response.Request, r.Response, cachecontrol.Options{})
	if err != nil {
		logger.Println(err)
	}
	if expires.IsZero() {
		//if time is zero than set it to current time plus 24 hours.
		r.Expires = time.Now().UTC().Add(time.Hour * 24)
	} else {
		r.Expires = expires
	}
	r.ReasonsNotToCache = reasons
}

//GetExpires returns Response Expires value.
func (r BaseFetcherResponse) GetExpires() time.Time {
	return r.Expires
}

//GetReasonsNotToCache returns an array of reasons why a response should not be cached.
func (r BaseFetcherResponse) GetReasonsNotToCache() []cacheobject.Reason {
	return r.ReasonsNotToCache
}

//GetExpires returns URL to be downloaded
func (r BaseFetcherRequest) GetURL() string {
	return strings.TrimSpace(strings.TrimRight(r.URL, "/"))
}

//Validate validates request to be send, prior to sending.
func (r BaseFetcherRequest) Validate() error {
	reqURL := strings.TrimSpace(r.URL)
	if _, err := url.ParseRequestURI(reqURL); err != nil {
		return &errs.BadRequest{err}
	}
	return nil
}
