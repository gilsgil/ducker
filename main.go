package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/tebeka/selenium"
	"github.com/tebeka/selenium/chrome"
)

func duckduckgoDorking(query string, period string, maxClicks int, verbose bool) {
	const port = 9515

	service, err := selenium.NewChromeDriverService("chromedriver", port)
	if err != nil {
		log.Fatalf("Error starting the ChromeDriver service: %v", err)
	}
	defer service.Stop()

	caps := selenium.Capabilities{"browserName": "chrome"}
	chromeCaps := chrome.Capabilities{
		Args: []string{
			"--headless",
			"--no-sandbox",
			"--disable-dev-shm-usage",
		},
	}
	caps.AddChrome(chromeCaps)

	wd, err := selenium.NewRemote(caps, fmt.Sprintf("http://localhost:%d/wd/hub", port))
	if err != nil {
		log.Fatalf("Error connecting to the WebDriver: %v", err)
	}
	defer wd.Quit()

	escapedQuery := url.QueryEscape(query)
	searchURL := fmt.Sprintf("https://duckduckgo.com/?q=%s&t=h_&ia=web", escapedQuery)
	if period != "" {
		searchURL = fmt.Sprintf("%s&df=%s", searchURL, period)
	}

	if err := wd.Get(searchURL); err != nil {
		log.Fatalf("Error loading the page: %v", err)
	}

	time.Sleep(5 * time.Second)

	allLinks := make(map[string]bool)
	clickCount := 0

	for clickCount < maxClicks {
		elems, err := wd.FindElements(selenium.ByCSSSelector, "a[data-testid='result-title-a']")
		if err != nil {
			if verbose {
				fmt.Println("Error finding result elements:", err)
			}
			break
		}

		for _, elem := range elems {
			link, err := elem.GetAttribute("href")
			if err != nil {
				continue
			}
			if link != "" {
				if _, exists := allLinks[link]; !exists {
					allLinks[link] = true
					fmt.Println(link)
				}
			}
		}

		moreResults, err := wd.FindElement(selenium.ByID, "more-results")
		if err != nil {
			if verbose {
				fmt.Println("The 'More results' button was not found or is no longer available.")
			}
			break
		}

		_, err = wd.ExecuteScript("arguments[0].click();", []interface{}{moreResults})
		if err != nil {
			if verbose {
				fmt.Println("Error clicking the 'More results' button:", err)
			}
			break
		}
		if verbose {
			fmt.Println("Clicking the 'More results' button to load additional results...")
		}
		clickCount++
		time.Sleep(4 * time.Second)
	}
}

func main() {
	query := flag.String("q", "", "Query to search on DuckDuckGo")
	clicks := flag.Int("c", 10, "Maximum number of clicks to load more results (default: 10)")
	verbose := flag.Bool("v", false, "Show additional status messages")

	day := flag.Bool("day", false, "Search results from the last day")
	week := flag.Bool("week", false, "Search results from the last week")
	month := flag.Bool("month", false, "Search results from the last month")
	year := flag.Bool("year", false, "Search results from the last year")

	flag.Parse()

	if *query == "" {
		fmt.Println("A query must be provided using -q or --query")
		flag.Usage()
		os.Exit(1)
	}

	var period string
	if *day {
		period = "d"
	} else if *week {
		period = "w"
	} else if *month {
		period = "m"
	} else if *year {
		period = "y"
	}

	duckduckgoDorking(*query, period, *clicks, *verbose)
}
