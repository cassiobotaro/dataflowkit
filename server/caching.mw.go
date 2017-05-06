package server

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"fmt"

	"github.com/pquerna/cachecontrol/cacheobject"
	"github.com/slotix/dataflowkit/cache"
	"github.com/slotix/dataflowkit/parser"
	"github.com/slotix/dataflowkit/splash"
	"github.com/spf13/viper"
)

func cachingMiddleware() ServiceMiddleware {
	return func(next ParseService) ParseService {
		return cachemw{next}
	}
}

type cachemw struct {
	ParseService
}

var redisCon cache.RedisConn

func (mw cachemw) Fetch(req splash.Request) (output io.ReadCloser, err error) {
	debug := true
	redisURL := viper.GetString("redis")
	redisPassword := ""
	redisCon = cache.NewRedisConn(redisURL, redisPassword, "", 0)

	redisValue, err := redisCon.GetValue(req.URL)
	if err == nil {
		var sResponse splash.Response
		if err := json.Unmarshal(redisValue, &sResponse); err != nil {
			logger.Println("Json Unmarshall error", err)
		}
		//Error responses: a 404 (Not Found) may be cached.
		if sResponse.Response.Status == 404 {
			return nil, fmt.Errorf("Error: 404. NOT FOUND")
		}
		output, err = sResponse.GetContent()
		if err != nil {
			logger.Printf(err.Error())
		}
		return output, err
	}

	resp, respErr := mw.ParseService.GetResponse(req)
	if respErr != nil {
		return nil, respErr
	}
	//Check if it is cacheable
	rv := cacheable(resp)
	expTime := rv.OutExpirationTime.Unix()
	if rv.OutExpirationTime.IsZero() {
		expTime = 0
	}
	if debug {
		if rv.OutErr != nil {
			logger.Println("Errors: ", rv.OutErr)
		}
		if rv.OutReasons != nil {
			logger.Println("Reasons to not cache: ", rv.OutReasons)
		}
		if rv.OutWarnings != nil {
			logger.Println("Warning headers to add: ", rv.OutWarnings)
		}
		logger.Println("Expiration: ", rv.OutExpirationTime.String())
	}
	//write data to cache
	if len(rv.OutReasons) == 0 {
		response, err := json.Marshal(resp)
		if err != nil {
			logger.Printf(err.Error())
		}
		err = redisCon.SetValue(req.URL, response)
		if err != nil {
			logger.Println(err.Error())
		}
		err = redisCon.SetExpireAt(req.URL, expTime)
		if err != nil {
			logger.Println(err.Error())
		}
	}
	if respErr != nil {
		return nil, respErr
	}

	output, err = resp.GetContent()
	if err != nil {
		return nil, err
	}
	return
}

func (mw cachemw) ParseData(payload []byte) (output []byte, err error) {
	redisURL := viper.GetString("redis")
	redisPassword := ""
	redisCon = cache.NewRedisConn(redisURL, redisPassword, "", 0)
	p, err := parser.NewParser(payload)
	if err != nil {
		return nil, err
	}
	redisKey := fmt.Sprintf("%s-%s", p.Format, p.PayloadMD5)
	redisValue, err := redisCon.GetValue(redisKey)
	if err == nil {
		return redisValue, nil
	}

	output, err = mw.ParseService.ParseData(payload)
	if err != nil {
		return nil, err
	}
	err = redisCon.SetValue(redisKey, output)
	if err != nil {
		logger.Println(err.Error())
	}
	err = redisCon.SetExpireIn(redisKey, 3600)
	if err != nil {
		logger.Println(err.Error())
	}
	return
}

//Cacheable check if resource is cacheable
func cacheable(resp *splash.Response) (rv cacheobject.ObjectResults) {

	respHeader := resp.Response.Headers.(http.Header)
	reqHeader := resp.Request.Headers.(http.Header)

	reqDir, err := cacheobject.ParseRequestCacheControl(reqHeader.Get("Cache-Control"))
	if err != nil {
		logger.Printf(err.Error())
	}
	resDir, err := cacheobject.ParseResponseCacheControl(respHeader.Get("Cache-Control"))
	if err != nil {
		logger.Printf(err.Error())
	}
	//logger.Println(respHeader)
	expiresHeader, _ := http.ParseTime(respHeader.Get("Expires"))
	dateHeader, _ := http.ParseTime(respHeader.Get("Date"))
	lastModifiedHeader, _ := http.ParseTime(respHeader.Get("Last-Modified"))
	obj := cacheobject.Object{
		//	CacheIsPrivate:         false,
		RespDirectives:         resDir,
		RespHeaders:            respHeader,
		RespStatusCode:         resp.Response.Status,
		RespExpiresHeader:      expiresHeader,
		RespDateHeader:         dateHeader,
		RespLastModifiedHeader: lastModifiedHeader,

		ReqDirectives: reqDir,
		ReqHeaders:    reqHeader,
		ReqMethod:     resp.Request.Method,

		NowUTC: time.Now().UTC(),
	}

	rv = cacheobject.ObjectResults{}
	cacheobject.CachableObject(&obj, &rv)
	cacheobject.ExpirationObject(&obj, &rv)
	return rv
}
