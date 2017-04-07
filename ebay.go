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
)

type SortOrderType string

const (
	SORT_DEFAULT                     = SortOrderType("")
	SORT_BEST_MATCH                  = SortOrderType("BestMatch")
	SORT_BID_COUNT_FEWEST            = SortOrderType("BidCountFewest")
	SORT_BID_COUNT_MOST              = SortOrderType("BidCountMost")
	SORT_COUNTRY_ASCENDING           = SortOrderType("CountryAscending")
	SORT_COUNTRY_DESCENDING          = SortOrderType("CountryDescending")
	SORT_CURRENT_PRICE_HIGHEST       = SortOrderType("CurrentPriceHighest")
	SORT_DISTANCE_NEAREST            = SortOrderType("DistanceNearest")
	SORT_END_TIME_SOONEST            = SortOrderType("EndTimeSoonest")
	SORT_PRICE_PLUS_SHIPPING_HIGHEST = SortOrderType("PricePlusShippingHighest")
	SORT_PRICE_PLUS_SHIPPING_LOWEST  = SortOrderType("PricePlusShippingLowest")
	SORT_START_TIME_NEWEST           = SortOrderType("StartTimeNewest")
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

type getUrl func(string, string, ...FilterOption) (string, error)

func New(application_id string) *EBay {
	e := EBay{}
	e.ApplicationId = application_id
	e.HttpRequest = httprequest.NewWithDefaults()
	return &e
}

type filterList struct {
	list           url.Values
	itemFilter     int
	outputSelector int
}

func newFilterList() *filterList {
	return &filterList{list: url.Values{}}
}

func (f *filterList) addFilter(name string, value string) {
	f.list.Add(name, value)
}

func (f *filterList) addItemFilter(name string, values ...string) {
	item := fmt.Sprintf("itemFilter(%v)", f.itemFilter)
	f.itemFilter += 1

	f.list.Add(item+".name", name)
	for i, v := range values {
		f.list.Add(fmt.Sprintf("%s.value(%v)", item, i), v)
	}
}

func (f *filterList) addOutputSelector(values ...string) {
	for _, v := range values {
		f.list.Add(fmt.Sprintf("outputSelector(%v)", f.outputSelector), v)
		f.outputSelector += 1
	}
}

func (e *EBay) build_sold_url(global_id, keywords string, options ...FilterOption) (string, error) {
	filters := newFilterList()
	filters.addItemFilter("Condition", "Used", "Unspecified")
	filters.addItemFilter("SoldItemsOnly", "true")

	return e.build_url(global_id, keywords, "findCompletedItems", filters, options...)
}

func (e *EBay) build_search_url(global_id, keywords string, options ...FilterOption) (string, error) {
	filters := newFilterList()
	filters.addItemFilter("ListingType", "FixedPrice", "AuctionWithBIN", "Auction")
	filters.addOutputSelector("SellerInfo")

	return e.build_url(global_id, keywords, "findItemsByKeywords", filters, options...)
}

func (e *EBay) build_url(global_id, keywords, operationName string, filters *filterList, options ...FilterOption) (string, error) {
	var u *url.URL
	u, err := url.Parse("http://svcs.ebay.com/services/search/FindingService/v1")
	if err != nil {
		return "", err
	}

	for _, o := range options {
		o(filters)
	}

	params := filters.list
	params.Add("OPERATION-NAME", operationName)
	params.Add("SERVICE-VERSION", "1.0.0")
	params.Add("SECURITY-APPNAME", e.ApplicationId)
	params.Add("GLOBAL-ID", global_id)
	params.Add("RESPONSE-DATA-FORMAT", "XML")
	params.Add("REST-PAYLOAD", "")
	params.Add("keywords", keywords)
	u.RawQuery = params.Encode()
	return u.String(), err
}

func (e *EBay) findItems(global_id string, keywords string, getUrl getUrl, options ...FilterOption) (FindItemsResponse, error) {
	var response FindItemsResponse
	url, err := getUrl(global_id, keywords, options...)
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

type FilterOption func(*filterList)

func SortOrder(sort_order SortOrderType) FilterOption {
	return func(f *filterList) {
		f.addFilter("sortOrder", string(sort_order))
	}
}

func PageNumber(page_number int) FilterOption {
	return func(f *filterList) {
		if page_number > 0 {
			f.addFilter("paginationInput.pageNumber", strconv.Itoa(page_number))
		}
	}
}

func PageSize(page_size int) FilterOption {
	return func(f *filterList) {
		if page_size > 0 {
			f.addFilter("paginationInput.entriesPerPage", strconv.Itoa(page_size))
		}
	}
}

func MinPrice(price float64) FilterOption {
	return func(f *filterList) {
		f.addItemFilter("MinPrice", fmt.Sprintf("%v", price))
	}
}

func MaxPrice(price float64) FilterOption {
	return func(f *filterList) {
		if price > 0.0 {
			f.addItemFilter("MaxPrice", fmt.Sprintf("%v", price))
		}
	}
}

func (e *EBay) FindItemsByKeywords(global_id, keywords string, options ...FilterOption) (FindItemsResponse, error) {
	return e.findItems(global_id, keywords, e.build_search_url, options...)
}

func (e *EBay) FindSoldItems(global_id, keywords string, options ...FilterOption) (FindItemsResponse, error) {
	return e.findItems(global_id, keywords, e.build_sold_url, options...)
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
