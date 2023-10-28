package main 

import (
	"fmt"
	"log"
	"net/http"
	"encoding/json"
	"encoding/xml"
	"time"
	"os"
	"flag"
	"sync"
)

type JsonResponse struct {
	Data struct {
		Nodes []Node `json:"nodes"`
	} `json:"data"`
}

type Node struct {
	ID string `json:"id"`
	Title string `json:"title"`
	Published string `json:"publish_at"`
	SeoFields struct {
		Slug string `json:"slug"`
	} `json:"seo_fields"`
	Image struct {
		Url string `json:"original_url"`
	} `json:"featured_image"`
}

type RssFeed struct {
	XMLName xml.Name `xml:"rss"`
	Version string `xml:"version,attr"`
	Atom string `xml:"xmlns:atom,attr"`
	Channel struct {
	    AtomLink struct {
    		Rel string `xml:"rel,attr"`
    		Href string `xml:"href,attr"`
    	} `xml:"atom:link"`
		Title string `xml:"title"`
	    Link string `xml:"link"`
	    Desc string `xml:"description"`
	    Item []RssItem `xml:"item"`
	} `xml:"channel"`
}

type RssItem struct {
    Title string `xml:"title"`
    Link string `xml:"link"`
    Guid struct {
    	Content string `xml:",chardata"`
    	IsPermaLink bool `xml:"isPermaLink,attr"`
    } `xml:"guid"`
    PubDate string `xml:"pubDate"`
    Enclosure struct {
    	Url string `xml:"url,attr"`
    	Length int64 `xml:"length,attr"`
    	Type string `xml:"type,attr"`
    } `xml:"enclosure"`
}

type Config struct {
	Url string `json:"url"`
	ThumbnailCompression string `json:"thumbnail_compression"`
	Interval time.Duration `json:"interval"`
}

func JsonToRssItem(node Node) (RssItem) {

	// Change time format into RSS standard (RFC 2822)
	timezone, _ := time.LoadLocation("UTC")
	okoTimeFormat, _ := time.ParseInLocation("2006-01-02T15:04:05", node.Published, timezone)
	rssTimeFormat := okoTimeFormat.Format("02 Jan 2006 15:04 -0700")

	link := "https://oko.press/" + node.SeoFields.Slug
	
	item := RssItem {
		Title: node.Title,
		Link: link,
		PubDate: rssTimeFormat,
	}

	var guid = &item.Guid
	guid.Content = node.ID
	guid.IsPermaLink = false

	var enclosure = &item.Enclosure
	imageUrl := config.ThumbnailCompression + node.Image.Url
	
	enclosure.Url = imageUrl
	enclosure.Length = 0
	enclosure.Type = "image/jpeg"

	return item
} 

func OkoPressRss() (string) {

	// Send GET request
	log.Println("Fetching OKO.press API")
	httpResponse, err := http.Get(config.Url)
	if err != nil {
		log.Panicf("Error while fetching URL: %s", err)
	}
	defer httpResponse.Body.Close()

	// Check server response
	if httpResponse.StatusCode != http.StatusOK {
		log.Panicf("Error: bad HTTP status: %s, URL: %s", httpResponse.Status, httpResponse.Request.URL)
	}

	// Parse JSON from response into struct
	var jsonBody JsonResponse
	parser := json.NewDecoder(httpResponse.Body)
	err = parser.Decode(&jsonBody)
	if err != nil {
		log.Panic("Error while parsing HAR file into JSON: ", err)
	}

	// Create RSS feed and add values
	var rss RssFeed
	rss.Version = "2.0"
	rss.Atom = "http://www.w3.org/2005/Atom"
	
	var channel = &rss.Channel
	channel.Title = "OKO.press"
	channel.Link = "https://oko.press"
	channel.AtomLink.Href = channel.Link
	channel.AtomLink.Rel = "self"
	channel.Desc = "OKO.press to portal informacyjny, który publikuje najnowsze wiadomości z różnych dziedzin: polityki, gospodarki, sportu, kultury, nauki i nauki. Znajdziesz tu także wywiady, analizy, sondaże, podcasty i multimedia."

	// Loop over nodes and add them to RSS struct
	var rssItems []RssItem
	var nodes = jsonBody.Data.Nodes
	for i := 0; i < len(nodes); i++ {
		item := JsonToRssItem(nodes[i])
		rssItems = append(rssItems, item)
	}
	channel.Item = rssItems

	// Struct to XML
	xmlExport, err := xml.MarshalIndent(rss, "", " ")
	if err != nil {
		log.Panic("Error while parsing struct into XML: ", err)
	}

	// RSS feed to text, add comment when last updated
	xmlText := string(xmlExport)
	now := time.Now().Format("02 Jan 2006 15:04 -0700")
	feed := "<!-- Last updated: " + now + " -->\n" + xmlText

	log.Println("RSS feed generated")
	return feed
}

func cron(wg *sync.WaitGroup) {

	defer wg.Done()
	
	log.Printf("Counting %d seconds to exit", config.Interval)
	time.Sleep(config.Interval * time.Second)
	
	log.Println("Exiting")
	os.Exit(0)
}

func serveHttp(wg *sync.WaitGroup) {

	defer wg.Done()

	log.Println("Starting HTTP server")
	
	// Serve RSS feed at / path
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintln(w, feed)
	})
	
	err := http.ListenAndServe(":" + port, nil)
	if err != nil {
		log.Panic("Error while serving HTTP content: ", err)
	}
}

// Create some global variables
var config Config
var port string
var feed string

func main() {

	// Get info from command line parameters
	var configPath string
	
	usage := "Usage:\n\t-p, --port\tport number (default 8000)\n\t-c, --config\tconfig file path"
	flag.Usage = func() { fmt.Printf(usage) }

	flag.StringVar(&port, "p", "8000", "")
	flag.StringVar(&port, "port", "8000", "")
	flag.StringVar(&configPath, "c", "NO_CONFIG", "")
	flag.StringVar(&configPath, "config", "NO_CONFIG", "")
	flag.Parse()

	// Check if config file was specified
	if configPath == "NO_CONFIG" {
		fmt.Printf("Please specify config path!")
		return
	}

	// Open config file 
	file, err := os.Open(configPath)
	if err != nil {
		log.Panic("Error while opening file: ", err)
	}
	defer file.Close()

	// Parse config file into struct
	configParser := json.NewDecoder(file)
	err = configParser.Decode(&config)
	if err != nil {
		log.Panic("Error while parsing config file into struct: ", err)
	}
	
	feed = OkoPressRss()

	// Run 2 concurrent functions: HTTP server and countdown to exit to OS
	var wg sync.WaitGroup
	wg.Add(2)
	go cron(&wg)
	go serveHttp(&wg)
	wg.Wait()
}
