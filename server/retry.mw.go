package server

////TODO: RetryTimes = 2
//Maximum number of times to retry, in addition to the first download.

//RETRY_HTTP_CODES
//Default: [500, 502, 503, 504, 408]

//Failed pages should be rescheduled for download at the end. once the spider has finished crawling all other (non failed) pages. 

func retryMiddleware() ServiceMiddleware {
	return func(next ParseService) ParseService {
		return retrymw{next}
	}
}

type retrymw struct {
	ParseService
}

func (mw retrymw) Download(url string) (output []byte, err error) {
	output, err = mw.ParseService.Download(url)
	if err != nil {
		logger.Println(err)
	}
	return
}
