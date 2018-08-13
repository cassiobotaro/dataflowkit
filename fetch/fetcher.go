package fetch

// The following code was sourced and modified from the
// https://github.com/andrew-d/goscrape package governed by MIT license.

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mafredri/cdp"
	"github.com/mafredri/cdp/devtool"
	"github.com/mafredri/cdp/protocol/dom"
	"github.com/mafredri/cdp/protocol/network"
	"github.com/mafredri/cdp/protocol/page"
	"github.com/mafredri/cdp/protocol/runtime"
	"github.com/mafredri/cdp/rpcc"
	"github.com/slotix/dataflowkit/errs"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
)

//Type represents types of fetcher
type Type string

//Fetcher types
const (
	//Base fetcher is used for downloading html web page using Go standard library's http
	Base Type = "Base"
	//Headless chrome is used to download content from JS driven web pages
	Chrome = "Chrome"
)

// Fetcher is the interface that must be satisfied by things that can fetch
// remote URLs and return their contents.
//
// Note: Fetchers may or may not be safe to use concurrently.  Please read the
// documentation for each fetcher for more details.
type Fetcher interface {
	//  Fetch is called to retrieve HTML content of a document from the remote server.
	Fetch(request Request) (io.ReadCloser, error)
	getCookieJar() http.CookieJar
	setCookieJar(jar http.CookieJar)
}

//Request struct contains request information sent to  Fetchers
type Request struct {
	Type string `json:"type"`
	//	URL to be retrieved
	URL string `json:"url"`
	//	HTTP method : GET, POST
	Method string
	// FormData is a string value for passing formdata parameters.
	//
	// For example it may be used for processing pages which require authentication
	//
	// Example:
	//
	// "auth_key=880ea6a14ea49e853634fbdc5015a024&referer=http%3A%2F%2Fexample.com%2F&ips_username=user&ips_password=userpassword&rememberMe=1"
	//
	FormData string `json:"formData,omitempty"`
	//UserToken identifies user to keep personal cookies information.
	UserToken string `json:"userToken"`
	//InfiniteScroll option is used for fetching web pages with Continuous Scrolling
	InfiniteScroll bool `json:"infiniteScroll"`
}

// BaseFetcher is a Fetcher that uses the Go standard library's http
// client to fetch URLs.
type BaseFetcher struct {
	client *http.Client
	//jar    *cookiejar.Jar
}

// ChromeFetcher is used to fetch Java Script rendeded pages.
type ChromeFetcher struct {
	cdpClient *cdp.Client
	client    *http.Client
	//jar       *cookiejar.Jar
}

//newFetcher creates instances of Fetcher for downloading a web page.
func newFetcher(t Type) Fetcher {
	switch t {
	case Base:
		return newBaseFetcher()
	case Chrome:
		return newChromeFetcher()
	default:
		logger.Panicf("unhandled type: %#v", t)
	}
	panic("unreachable")
}

// newBaseFetcher creates instances of newBaseFetcher{} to fetch
// a page content from regular websites as-is
// without running js scripts on the page.
func newBaseFetcher() *BaseFetcher {
	var client *http.Client
	proxy := viper.GetString("PROXY")
	if len(proxy) > 0 {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			logger.Error(err)
			return nil
		}
		transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
		client = &http.Client{Transport: transport}
	} else {
		client = &http.Client{}
	}
	f := &BaseFetcher{
		client: client,
	}
	return f
}

// Fetch retrieves document from the remote server. It returns web page content along with cache and expiration information.
func (bf *BaseFetcher) Fetch(request Request) (io.ReadCloser, error) {
	resp, err := bf.response(request)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

//Response return response after document fetching using BaseFetcher
func (bf *BaseFetcher) response(r Request) (*http.Response, error) {
	//URL validation
	if _, err := url.ParseRequestURI(r.getURL()); err != nil {
		return nil, &errs.BadRequest{err}
	}

	// if bf.jar != nil {
	// 	bf.client.Jar = bf.jar
	// }

	var err error
	var req *http.Request
	var resp *http.Response

	if r.FormData == "" {
		req, err = http.NewRequest(r.Method, r.URL, nil)
		if err != nil {
			return nil, err
		}
	} else {
		//if form data exists send POST request
		formData := parseFormData(r.FormData)
		req, err = http.NewRequest("POST", r.URL, strings.NewReader(formData.Encode()))
		if err != nil {
			return nil, err
		}
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Add("Content-Length", strconv.Itoa(len(formData.Encode())))
	}

	resp, err = bf.client.Do(req)
	if err != nil {
		return nil, &errs.BadRequest{err}
	}
	if resp.StatusCode != 200 {
		switch resp.StatusCode {
		case 404:
			return nil, &errs.NotFound{r.URL}
		case 403:
			return nil, &errs.Forbidden{r.URL}
		case 400:
			return nil, &errs.BadRequest{err}
		case 401:
			return nil, &errs.Unauthorized{}
		case 407:
			return nil, &errs.ProxyAuthenticationRequired{}
		case 500:
			return nil, &errs.InternalServerError{}
		case 502:
			return nil, &errs.BadGateway{}
		case 504:
			return nil, &errs.GatewayTimeout{}
		default:
			return nil, &errs.Error{"Unknown Error"}
		}
	}
	return resp, err
}

func (bf *BaseFetcher) getCookieJar() http.CookieJar { //*cookiejar.Jar {
	return bf.client.Jar
}

//func (bf *BaseFetcher) setCookieJar(jar *cookiejar.Jar) {
func (bf *BaseFetcher) setCookieJar(jar http.CookieJar) {

	bf.client.Jar = jar
}

// parseFormData is used for converting formdata string to url.Values type
func parseFormData(fd string) url.Values {
	//"auth_key=880ea6a14ea49e853634fbdc5015a024&referer=http%3A%2F%2Fexample.com%2F&ips_username=usr&ips_password=passw&rememberMe=0"
	formData := url.Values{}
	pairs := strings.Split(fd, "&")
	for _, pair := range pairs {
		kv := strings.Split(pair, "=")
		formData.Add(kv[0], kv[1])
	}
	return formData
}

// Static type assertion
var _ Fetcher = &BaseFetcher{}

// NewChromeFetcher returns ChromeFetcher
func newChromeFetcher() *ChromeFetcher {
	var client *http.Client
	proxy := viper.GetString("PROXY")
	if len(proxy) > 0 {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			logger.Error(err)
			return nil
		}
		transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
		client = &http.Client{Transport: transport}
	} else {
		client = &http.Client{}
	}
	f := &ChromeFetcher{
		client: client,
	}
	return f
}

// Fetch retrieves document from the remote server. It returns web page content along with cache and expiration information.
func (f *ChromeFetcher) Fetch(request Request) (io.ReadCloser, error) {
	//URL validation
	if _, err := url.ParseRequestURI(strings.TrimSpace(request.getURL())); err != nil {
		return nil, &errs.BadRequest{err}
	}
	// if f.jar != nil {
	// 	f.client.Jar = f.jar
	// }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	devt := devtool.New(viper.GetString("CHROME"), devtool.WithClient(f.client))
	//https://github.com/mafredri/cdp/issues/60
	//pt, err := devt.Get(ctx, devtool.Page)
	pt, err := devt.Create(ctx)
	if err != nil {
		return nil, err
	}
	// Connect to WebSocket URL (page) that speaks the Chrome Debugging Protocol.
	conn, err := rpcc.DialContext(ctx, pt.WebSocketDebuggerURL)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	defer conn.Close() // Cleanup.
	defer devt.Close(ctx, pt)
	// if err != nil {
	// 	return nil, err
	// }
	// Create a new CDP Client that uses conn.
	f.cdpClient = cdp.NewClient(conn)

	// Give enough capacity to avoid blocking any event listeners
	abort := make(chan error, 2)
	// Watch the abort channel.
	go func() {
		select {
		case <-ctx.Done():
		case err := <-abort:
			fmt.Printf("aborted: %s\n", err.Error())
			cancel()
		}
	}()
	// Setup event handlers early because domain events can be sent as
	// soon as Enable is called on the domain.
	// if err = abortOnErrors(ctx, c, scriptID, abort); err != nil {
	// 	fmt.Println(err)
	// 	return
	// }

	if err = runBatch(
		// Enable all the domain events that we're interested in.
		func() error { return f.cdpClient.DOM.Enable(ctx) },
		func() error { return f.cdpClient.Network.Enable(ctx, nil) },
		func() error { return f.cdpClient.Page.Enable(ctx) },
		func() error { return f.cdpClient.Runtime.Enable(ctx) },
	); err != nil {
		return nil, err
	}
	domLoadTimeout := 60 * time.Second
	if request.FormData == "" {
		err = f.navigate(ctx, f.cdpClient.Page, "GET", request.getURL(), "", domLoadTimeout)
	} else {
		formData := parseFormData(request.FormData)
		err = f.navigate(ctx, f.cdpClient.Page, "POST", request.getURL(), formData.Encode(), domLoadTimeout)
	}
	if err != nil {
		return nil, err
	}

	if request.InfiniteScroll {
		path := filepath.Join(viper.GetString("CHROME_SCRIPTS"), "scroll2bottom.js")
		err = f.runJSFromFile(ctx, path)
		if err != nil {
			return nil, err
		}
	}

	// Fetch the document root node. We can pass nil here
	// since this method only takes optional arguments.
	doc, err := f.cdpClient.DOM.GetDocument(ctx, nil)
	if err != nil {
		return nil, err
	}

	// Get the outer HTML for the page.
	result, err := f.cdpClient.DOM.GetOuterHTML(ctx, &dom.GetOuterHTMLArgs{
		NodeID: &doc.Root.NodeID,
	})
	if err != nil {
		return nil, err
	}
	readCloser := ioutil.NopCloser(strings.NewReader(result.OuterHTML))
	return readCloser, nil

}

func (f *ChromeFetcher) setCookieJar(jar http.CookieJar) {
	f.client.Jar = jar
}

func (f *ChromeFetcher) getCookieJar() http.CookieJar {
	return f.client.Jar
}

// Static type assertion
var _ Fetcher = &ChromeFetcher{}

// navigate to the URL and wait for DOMContentEventFired. An error is
// returned if timeout happens before DOMContentEventFired.
func (f *ChromeFetcher) navigate(ctx context.Context, pageClient cdp.Page, method, url string, formData string, timeout time.Duration) error {
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	// Make sure Page events are enabled.
	err := pageClient.Enable(ctx)
	if err != nil {
		return err
	}

	// Navigate to GitHub, block until ready.
	loadEventFired, err := pageClient.LoadEventFired(ctx)
	if err != nil {
		return err
	}

	if method == "GET" {
		_, err = pageClient.Navigate(ctx, page.NewNavigateArgs(url))
		if err != nil {
			return err
		}
	} else {
		pattern := network.RequestPattern{URLPattern: &url}
		patterns := []network.RequestPattern{pattern}

		interArgs := network.NewSetRequestInterceptionArgs(patterns)
		err = f.cdpClient.Network.SetRequestInterception(ctx, interArgs)
		if err != nil {
			return err
		}
		cl, err := f.cdpClient.Network.RequestIntercepted(ctx)
		if err != nil {
			panic(err)
		}
		kill := make(chan bool)
		go func(kill chan bool) {
			var sig = false
			for {
				if sig {
					return
				}
				select {
				case <-cl.Ready():
					r, err := cl.Recv()
					if err != nil {
						logger.Error(err)
						sig = true
						break
					}
					interceptedArgs := network.NewContinueInterceptedRequestArgs(r.InterceptionID)
					interceptedArgs.SetMethod("POST")
					interceptedArgs.SetPostData(formData)
					fData := fmt.Sprintf(`{"Content-Type":"application/x-www-form-urlencoded","Content-Length":%d}`, len(formData))
					interceptedArgs.Headers = []byte(fData)
					if err = f.cdpClient.Network.ContinueInterceptedRequest(ctx, interceptedArgs); err != nil {
						logger.Error(err)
						sig = true
						break
					}
				case <-kill:
					sig = true
					break
				}
			}
		}(kill)
		_, err = pageClient.Navigate(ctx, page.NewNavigateArgs(url))
		if err != nil {
			return err
		}
		kill <- true
	}
	_, err = loadEventFired.Recv()
	if err != nil {
		return err
	}
	loadEventFired.Close()
	time.Sleep(500 * time.Millisecond)
	return nil
}

func (f ChromeFetcher) runJSFromFile(ctx context.Context, path string) error {
	exp, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}

	compileReply, err := f.cdpClient.Runtime.CompileScript(context.Background(), &runtime.CompileScriptArgs{
		Expression:    string(exp),
		PersistScript: true,
	})
	if err != nil {
		panic(err)
	}
	awaitPromise := true

	_, err = f.cdpClient.Runtime.RunScript(ctx, &runtime.RunScriptArgs{
		ScriptID:     *compileReply.ScriptID,
		AwaitPromise: &awaitPromise,
	})
	return err
}

// removeNodes deletes all provided nodeIDs from the DOM.
// func removeNodes(ctx context.Context, domClient cdp.DOM, nodes ...dom.NodeID) error {
// 	var rmNodes []runBatchFunc
// 	for _, id := range nodes {
// 		arg := dom.NewRemoveNodeArgs(id)
// 		rmNodes = append(rmNodes, func() error { return domClient.RemoveNode(ctx, arg) })
// 	}
// 	return runBatch(rmNodes...)
// }

// runBatchFunc is the function signature for runBatch.
type runBatchFunc func() error

// runBatch runs all functions simultaneously and waits until
// execution has completed or an error is encountered.
func runBatch(fn ...runBatchFunc) error {
	eg := errgroup.Group{}
	for _, f := range fn {
		eg.Go(f)
	}
	return eg.Wait()
}

//GetURL returns URL to be fetched
func (req Request) getURL() string {
	return strings.TrimRight(strings.TrimSpace(req.URL), "/")
}

// Host returns Host value from Request
func (req Request) Host() (string, error) {
	u, err := url.Parse(req.getURL())
	if err != nil {
		return "", err
	}
	return u.Host, nil
}
