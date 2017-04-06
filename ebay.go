package ebay

import (
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/heatxsink/go-httprequest"
	"net/url"
	"strconv"
	"time"
)

const (
	GLOBAL_ID_EBAY_US = "EBAY-US"
	GLOBAL_ID_EBAY_FR = "EBAY-FR"
	GLOBAL_ID_EBAY_DE = "EBAY-DE"
	GLOBAL_ID_EBAY_IT = "EBAY-IT"
	GLOBAL_ID_EBAY_ES = "EBAY-ES"

	SORT_DEFAULT                     = ""
	SORT_BEST_MATCH                  = "BestMatch"
	SORT_BID_COUNT_FEWEST            = "BidCountFewest"
	SORT_BID_COUNT_MOST              = "BidCountMost"
	SORT_COUNTRY_ASCENDING           = "CountryAscending"
	SORT_COUNTRY_DESCENDING          = "CountryDescending"
	SORT_CURRENT_PRICE_HIGHEST       = "CurrentPriceHighest"
	SORT_DISTANCE_NEAREST            = "DistanceNearest"
	SORT_END_TIME_SOONEST            = "EndTimeSoonest"
	SORT_PRICE_PLUS_SHIPPING_HIGHEST = "PricePlusShippingHighest"
	SORT_PRICE_PLUS_SHIPPING_LOWEST  = "PricePlusShippingLowest"
	SORT_START_TIME_NEWEST           = "StartTimeNewest"
)

type Item struct {
	ItemId   string `xml:"itemId"`
	Title    string `xml:"title"`
	Location string `xml:"location"`
	// CurrentPrice  float64   `xml:"sellingStatus>currentPrice"`
	CurrentPrice  float64   `xml:"sellingStatus>convertedCurrentPrice"`
	ShippingPrice float64   `xml:"shippingInfo>shippingServiceCost"`
	BinPrice      float64   `xml:"listingInfo>buyItNowPrice"`
	ShipsTo       []string  `xml:"shippingInfo>shipToLocations"`
	ListingUrl    string    `xml:"viewItemURL"`
	ImageUrl      string    `xml:"galleryURL"`
	Site          string    `xml:"globalId"`
	EndTime       time.Time `xml:"listingInfo>endTime"`
	SellerInfo    Seller    `xml:"sellerInfo"`
}

type Seller struct {
	UserName      string  `xml:"sellerUserName"`
	FeedbackScore int64   `xml:"feedbackScore"`
	FeedbackPerc  float64 `xml:"positiveFeedbackPercent"`
}

type FindItemsResponse struct {
	XmlName      xml.Name `xml:"findItemsByKeywordsResponse"`
	Items        []Item   `xml:"searchResult>item"`
	Timestamp    string   `xml:"timestamp"`
	PageNumber   int      `xml:"paginationOutput>pageNumber"`
	TotalPages   int      `xml:"paginationOutput>totalPages"`
	TotalEntries int      `xml:"paginationOutput>totalEntries"`
}

type ErrorMessage struct {
	XmlName xml.Name `xml:"errorMessage"`
	Error   Error    `xml:"error"`
}

type Error struct {
	ErrorId   string `xml:"errorId"`
	Domain    string `xml:"domain"`
	Severity  string `xml:"severity"`
	Category  string `xml:"category"`
	Message   string `xml:"message"`
	SubDomain string `xml:"subdomain"`
}

type EBay struct {
	ApplicationId string
	HttpRequest   *httprequest.HttpRequest
}

type getUrl func(string, string, string, int, int) (string, error)

func New(application_id string) *EBay {
	e := EBay{}
	e.ApplicationId = application_id
	e.HttpRequest = httprequest.NewWithDefaults()
	return &e
}

func (e *EBay) build_sold_url(global_id string, keywords, sort_order string, entries_per_page, page_number int) (string, error) {
	filters := url.Values{}
	filters.Add("itemFilter(0).name", "Condition")
	filters.Add("itemFilter(0).value(0)", "Used")
	filters.Add("itemFilter(0).value(1)", "Unspecified")
	filters.Add("itemFilter(1).name", "SoldItemsOnly")
	filters.Add("itemFilter(1).value(0)", "true")

	if sort_order != "" {
		filters.Add("sortOrder", sort_order)
	}

	return e.build_url(global_id, keywords, "findCompletedItems", entries_per_page, page_number, filters)
}

func (e *EBay) build_search_url(global_id string, keywords, sort_order string, entries_per_page, page_number int) (string, error) {
	filters := url.Values{}
	filters.Add("itemFilter(0).name", "ListingType")
	filters.Add("itemFilter(0).value(0)", "FixedPrice")
	filters.Add("itemFilter(0).value(1)", "AuctionWithBIN")
	filters.Add("itemFilter(0).value(2)", "Auction")

	filters.Add("outputSelector(0)", "SellerInfo")

	if sort_order != "" {
		filters.Add("sortOrder", sort_order)
	}

	return e.build_url(global_id, keywords, "findItemsByKeywords", entries_per_page, page_number, filters)
}

func (e *EBay) build_url(global_id string, keywords string, operationName string, entries_per_page, page_number int, filters url.Values) (string, error) {
	var u *url.URL
	u, err := url.Parse("http://svcs.ebay.com/services/search/FindingService/v1")
	if err != nil {
		return "", err
	}
	params := url.Values{}
	params.Add("OPERATION-NAME", operationName)
	params.Add("SERVICE-VERSION", "1.0.0")
	params.Add("SECURITY-APPNAME", e.ApplicationId)
	params.Add("GLOBAL-ID", global_id)
	params.Add("RESPONSE-DATA-FORMAT", "XML")
	params.Add("REST-PAYLOAD", "")
	params.Add("keywords", keywords)
	params.Add("paginationInput.entriesPerPage", strconv.Itoa(entries_per_page))
	if page_number > 0 {
		params.Add("paginationInput.pageNumber", strconv.Itoa(page_number))
	}
	for key := range filters {
		for _, val := range filters[key] {
			params.Add(key, val)
		}
	}
	u.RawQuery = params.Encode()
	return u.String(), err
}

func (e *EBay) findItems(global_id string, keywords, sort_order string, entries_per_page, page_number int, getUrl getUrl) (FindItemsResponse, error) {
	var response FindItemsResponse
	url, err := getUrl(global_id, keywords, sort_order, entries_per_page, page_number)
	if err != nil {
		return response, err
	}
	headers := make(map[string]string)
	headers["User-Agent"] = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_7_3) AppleWebKit/535.11 (KHTML, like Gecko) Chrome/17.0.963.56 Safari/535.11"
	body, status_code, err := e.HttpRequest.Get(url, headers)
	if err != nil {
		return response, err
	}
	if status_code != 200 {
		var em ErrorMessage
		err = xml.Unmarshal([]byte(body), &em)
		if err != nil {
			return response, err
		}
		return response, errors.New(em.Error.Message)
	} else {
		err = xml.Unmarshal([]byte(body), &response)
		if err != nil {
			return response, err
		}
	}
	return response, err
}

func (e *EBay) FindItemsByKeywords(global_id string, keywords, sort_order string, entries_per_page, page_number int) (FindItemsResponse, error) {
	return e.findItems(global_id, keywords, sort_order, entries_per_page, page_number, e.build_search_url)
}

func (e *EBay) FindSoldItems(global_id string, keywords, sort_order string, entries_per_page, page_number int) (FindItemsResponse, error) {
	return e.findItems(global_id, keywords, sort_order, entries_per_page, page_number, e.build_sold_url)
}

func (r *FindItemsResponse) Dump() {
	fmt.Println("FindItemsResponse")
	fmt.Println("--------------------------")
	fmt.Println("Timestamp: ", r.Timestamp)
	fmt.Println("Items:")
	fmt.Println("------")
	for _, i := range r.Items {
		fmt.Println("Title: ", i.Title)
		fmt.Println("------")
		fmt.Println("\tListing Url:     ", i.ListingUrl)
		fmt.Println("\tBin Price:       ", i.BinPrice)
		fmt.Println("\tCurrent Price:   ", i.CurrentPrice)
		fmt.Println("\tBuy-it-now Price:", i.BinPrice)
		fmt.Println("\tShipping Price:  ", i.ShippingPrice)
		fmt.Println("\tShips To:        ", i.ShipsTo)
		fmt.Println("\tSeller Location: ", i.Location)
		fmt.Println("\tSeller Info:     ", i.SellerInfo)
		fmt.Println()
	}
}
