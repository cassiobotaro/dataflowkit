package fetch

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	//"github.com/juju/persistent-cookiejar"
	"net/http/cookiejar"

	"github.com/slotix/dataflowkit/storage"
	"github.com/spf13/viper"
	"golang.org/x/net/publicsuffix"
)

// Service defines Fetch service interface
type Service interface {
	Fetch(req Request) (io.ReadCloser, error)
}

// FetchService implements service with empty struct
type FetchService struct {
}

// ServiceMiddleware defines a middleware for a Fetch service
type ServiceMiddleware func(Service) Service

// Fetch method implements fetching content from web page with Base or Chrome fetcher.
func (fs FetchService) Fetch(req Request) (io.ReadCloser, error) {
	var fetcher Fetcher
	switch req.Type {
	case "chrome":
		fetcher = newFetcher(Chrome)
	default:
		fetcher = newFetcher(Base)
	}
	var (
		jar     http.CookieJar //*cookiejar.Jar
		cookies []byte
		cArr    []*http.Cookie
		s       storage.Store
	)

	jarOpts := &cookiejar.Options{PublicSuffixList: publicsuffix.List}
	jar, err := cookiejar.New(jarOpts)
	if err != nil {
		logger.Error("Failed to create Cookie Jar")

	}
	u, err := url.Parse(req.getURL())
	if err != nil {
		return nil, err
	}
	if req.UserToken != "" {
		storageType := viper.GetString("STORAGE_TYPE")
		s = storage.NewStore(storageType)
		cookies, err = s.Read(storage.Record{
			Type: storage.COOKIES,
			Key:  req.UserToken,
		})
		if err != nil {
			logger.Warningf("Failed to read cookie for %s. %s", req.UserToken, err.Error())
		}
		cArr = []*http.Cookie{}
		if len(cookies) != 0 {
			err = json.Unmarshal(cookies, &cArr)
			if err != nil {
				return nil, err
			}

			tempCarr := []*http.Cookie{}
			for i := 0; i < len(cArr); i++ {
				c := cArr[i]
				if u.Host == c.Domain {
					tempCarr = append(tempCarr, c)
					cArr = append(cArr[:i], cArr[i+1:]...)
					i--
				}
			}
			jar.SetCookies(u, tempCarr)
		}
	}
	fetcher.setCookieJar(jar)
	res, err := fetcher.Fetch(req)
	if err != nil {
		return nil, err
	}
	if req.UserToken != "" {
		jar = fetcher.getCookieJar()
		//cArr = append(cArr, jar.AllCookies()...)
		cArr = append(cArr, jar.Cookies(u)...)
		cookies, err = json.Marshal(cArr)
		if err != nil {
			return nil, err
		}
		err = s.Write(storage.Record{
			Type:    storage.COOKIES,
			Key:     req.UserToken,
			Value:   cookies,
			ExpTime: 0,
		})

		if err != nil {
			logger.Warningf("Failed to write cookie for %s. %s", req.UserToken, err.Error())
		}
		s.Close()
	}
	return res, nil
}
