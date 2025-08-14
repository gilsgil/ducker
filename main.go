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

func duckduckgoDorking(query string, period string, maxPages int, verbose bool) {
	const port = 9515

	svc, err := selenium.NewChromeDriverService("chromedriver", port)
	if err != nil {
		log.Fatalf("Error starting the ChromeDriver service: %v", err)
	}
	defer svc.Stop()

	caps := selenium.Capabilities{"browserName": "chrome"}
	chromeCaps := chrome.Capabilities{
		Args: []string{
			"--headless=new",
			"--no-sandbox",
			"--disable-dev-shm-usage",
			"--window-size=1280,1000",
			"--disable-blink-features=AutomationControlled",
			"--user-agent=Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
		},
		ExcludeSwitches: []string{"enable-automation"},
	}
	caps.AddChrome(chromeCaps)

	wd, err := selenium.NewRemote(caps, fmt.Sprintf("http://localhost:%d/wd/hub", port))
	if err != nil {
		log.Fatalf("Error connecting to the WebDriver: %v", err)
	}
	defer wd.Quit()

	escaped := url.QueryEscape(query)
	searchURL := fmt.Sprintf("https://duckduckgo.com/?q=%s&t=h_&ia=web", escaped)
	if period != "" {
		searchURL += "&df=" + period // d,w,m,y
	}
	if err := wd.Get(searchURL); err != nil {
		log.Fatalf("Error loading the page: %v", err)
	}

	// Espera os primeiros resultados aparecerem
	selector := "h2 > a, a[data-testid='result-title-a']"
	if err := wd.WaitWithTimeout(func(w selenium.WebDriver) (bool, error) {
		els, _ := w.FindElements(selenium.ByCSSSelector, selector)
		return len(els) > 0, nil
	}, 15*time.Second); err != nil {
		if verbose {
			title, _ := wd.Title()
			html, _ := wd.PageSource()
			fmt.Println("[warn] Nenhum resultado visível após timeout. Title:", title)
			fmt.Println("[debug] Primeiros 500 chars do HTML:", runesPrefix(html, 500))
		}
	}

	all := make(map[string]bool)
	prevCount := 0

	for page := 0; page < maxPages; page++ {
		// Coleta dos links da página atual
		els, _ := wd.FindElements(selenium.ByCSSSelector, selector)
		for _, e := range els {
			href, _ := e.GetAttribute("href")
			if href != "" && !all[href] {
				all[href] = true
				fmt.Println(href)
			}
		}

		// Scroll infinito — rola até o fim e aguarda novos resultados
		_, _ = wd.ExecuteScript("window.scrollTo(0, document.body.scrollHeight);", nil)

		// Aguarda aumento na contagem de resultados (~6s)
		increased := false
		for i := 0; i < 12; i++ {
			time.Sleep(500 * time.Millisecond)
			cur, _ := wd.FindElements(selenium.ByCSSSelector, selector)
			if len(cur) > prevCount {
				prevCount = len(cur)
				increased = true
				break
			}
		}
		if verbose {
			if increased {
				fmt.Printf("[info] Página lógica %d carregada, total de elementos: %d\n", page+1, prevCount)
			} else {
				fmt.Println("[info] Parece que não há mais resultados para carregar.")
			}
		}
		if !increased {
			break
		}
	}
}

// helper para truncar impressão de HTML no -v
func runesPrefix(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n])
}

func main() {
	query := flag.String("q", "", "Query to search on DuckDuckGo")
	clicks := flag.Int("c", 10, "Max scroll loads to fetch more results (default: 10)")
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
