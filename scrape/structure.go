package scrape

import (
	"github.com/slotix/dataflowkit/splash"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/slotix/dataflowkit/extract"
	"github.com/slotix/dataflowkit/paginate"
	"github.com/temoto/robotstxt"
)

// Extractor type represents Extractor types available for scraping.
// Here is the list of Extractor types are currently supported:
// text, html, outerHtml, attr, link, image, regex, const, count
// Find more actual information in docs/extractors.md
type Extractor struct {
	Type string `json:"type"`
	// Params are unique for definite type
	Params interface{} `json:"params"`
}

type field struct {
	Name     string `json:"name"`
	Selector string `json:"selector"`
	//Count     int       `json:"count"`
	Extractor Extractor `json:"extractor"`
	Details   *details  `json:"details"`
}

type details struct {
	Fields []field `json:"fields"`
}

type paginator struct {
	Selector  string `json:"selector"`
	Attribute string `json:"attr"`
	MaxPages  int    `json:"maxPages"`
}

// Find more actual information in docs/payload.md
type Payload struct {
	// Name - Collection name.
	Name string `json:"name"`
	//Request             splash.Request `json:"request"`
	Request interface{} `json:"request"`
	//A set of fields used to extract data from a web page.
	Fields []field `json:"fields"`
	//PayloadMD5 encodes payload content to MD5. It is used for generating file name to be stored.
	PayloadMD5 []byte `json:"payloadMD5"`
	// The following Output formats are available: CSV, JSON, Microsoft Excel, XML
	Format string `json:"format"`
	//Paginator is used to scrape multiple pages.
	Paginator *paginator `json:"paginator"`
	//Paginated results are returned if *paginateResults* is *true*.
	//Default value is false
	PaginateResults     *bool         `json:"paginateResults"`
	FetchDelay          time.Duration `json:"fetchDelay"`
	RandomizeFetchDelay *bool         `json:"randomizeFetchDelay"`
	RetryTimes          int           `json:"retryTimes"`
}

// The DividePageFunc type is used to extract a page's blocks during a scrape.
// For more information, please see the documentation on the ScrapeConfig type.
type DividePageFunc func(*goquery.Selection) []*goquery.Selection

// A Part represents a given chunk of data that is to be extracted from every
// block in each page of a scrape.
type Part struct {
	// The name of this part.  Required, and will be used to aggregate results.
	Name string

	// A sub-selector within the given block to process.  Pass in "." to use
	// the root block's selector with no modification.
	Selector string

	// Extractor contains the logic on how to extract some results from the
	// selector that is provided to this Piece.
	Extractor extract.Extractor

	Details *Scraper
}

//Scraper struct consolidates settings for scraping task.
type Scraper struct {
	Request splash.Request
	// Paginator is the Paginator to use for this current scrape.
	//
	// If Paginator is nil, then no pagination is performed and it is assumed that
	// the initial URL is the only page.
	Paginator paginate.Paginator

	// DividePage splits a page into individual 'blocks'.  When scraping, we treat
	// each page as if it contains some number of 'blocks', each of which can be
	// further subdivided into what actually needs to be extracted.
	//
	// If the DividePage function is nil, then no division is performed and the
	// page is assumed to contain a single block containing the entire <body>
	// tag.
	DividePage DividePageFunc

	// Parts contains the list of data that is extracted for each block.  For
	// every block that is the result of the DividePage function (above), all of
	// the Parts entries receives the selector representing the block, and can
	// return a result.  If the returned result is nil, then the Part is
	// considered not to exist in this block, and is not included.
	//
	// Note: if a Part's Extractor returns an error, it results in the scrape
	// being aborted - this can be useful if you need to ensure that a given Part
	// is required, for example.
	Parts []Part
	//Opts contains options that are used during the progress of a
	// scrape.
	Opts ScrapeOptions
}

// Results describes the results of a scrape.  It contains a list of all
// pages (URLs) visited during the process, along with all results generated
// from each Part in each page.
type Results struct {
	// Visited contain a map[url]error during this scrape.
	// Always contains at least one element - the initial URL.
	//Failed pages should be rescheduled for download at the end if during a scrape one of the following statuses returned [500, 502, 503, 504, 408]
	//once the spider has finished crawling all other (non failed) pages.
	Visited map[string]error

	// Output represents combined results after parsing from each Part of each page.  Essentially, the top-level array
	// is for each page, the second-level array is for each block in a page, and
	// the final map[string]interface{} is the mapping of Part.Name to results.
	Output [][]map[string]interface{}
}

// Task keeps Results of Task generated from Payload along with other auxiliary information
type Task struct {
	ID       string
	Payload  Payload
	Scrapers []*Scraper
	//Tasks []*Task
	Robots map[string]*robotstxt.RobotsData
	Results
	//Cookies string
}
