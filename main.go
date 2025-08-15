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

	// Seletor resiliente para os links de resultado
	selector := "h2 > a, a[data-testid='result-title-a']"

	// Espera primeiros resultados
	if err := wd.WaitWithTimeout(func(w selenium.WebDriver) (bool, error) {
		els, _ := w.FindElements(selenium.ByCSSSelector, selector)
		return len(els) > 0, nil
	}, 15*time.Second); err != nil && verbose {
		title, _ := wd.Title()
		html, _ := wd.PageSource()
		fmt.Println("[warn] Nenhum resultado visível após timeout. Title:", title)
		fmt.Println("[debug] Primeiros 500 chars do HTML:", runesPrefix(html, 500))
	}

	all := make(map[string]bool)
	prevCount := 0

	for page := 0; page < maxPages; page++ {
		// Coleta links atuais (imprime só novos)
		els, _ := wd.FindElements(selenium.ByCSSSelector, selector)
		for _, e := range els {
			href, _ := e.GetAttribute("href")
			if href != "" && !all[href] {
				all[href] = true
				fmt.Println(href)
			}
		}
		prevCount = len(els)

		// Tentar carregar mais (botão OU scroll incremental)
		clicked := tryLoadMore(wd, verbose)

		// Espera subir a contagem de resultados (até ~8s)
		increased := waitGrowth(wd, selector, prevCount, 8*time.Second)

		if verbose {
			switch {
			case increased:
				fmt.Printf("[info] Mais resultados carregados (iter %d). total elems: %d\n", page+1, prevCount)
			case clicked:
				fmt.Println("[info] Cliquei no botão de mais resultados, mas não vi aumento. Vou parar.")
			default:
				fmt.Println("[info] Não há botão e o scroll não aumentou a contagem. Fim.")
			}
		}
		if !increased {
			break
		}
	}
}

// aguarda aumento da contagem de elementos que casam com o seletor
func waitGrowth(wd selenium.WebDriver, selector string, prev int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		cur, _ := wd.FindElements(selenium.ByCSSSelector, selector)
		if len(cur) > prev {
			return true
		}
	}
	return false
}

// tenta clicar no botão "More results"; se não existir, faz scroll incremental até o fim
func tryLoadMore(wd selenium.WebDriver, verbose bool) bool {
	// 1) Diversos seletores possíveis do botão
	btnSelectors := []string{
		"#more-results",
		"a.result--more__btn",
		"button.result--more__btn",
		"a[aria-label*='More']",
		"button[aria-label*='More']",
		"button:has(span:contains('More'))", // pode não funcionar em todos os engines, mas tentamos
	}

	for _, q := range btnSelectors {
		btns, _ := wd.FindElements(selenium.ByCSSSelector, q)
		if len(btns) > 0 {
			btn := btns[0]
			_, _ = wd.ExecuteScript("arguments[0].scrollIntoView({block:'center'});", []interface{}{btn})
			// tenta click normal
			if err := btn.Click(); err != nil {
				// força via JS se o click normal falhar
				_, _ = wd.ExecuteScript("arguments[0].click();", []interface{}{btn})
			}
			if verbose {
				fmt.Println("[info] Cliquei no botão de mais resultados:", q)
			}
			// espera um pouco para o carregamento começar
			time.Sleep(1200 * time.Millisecond)
			return true
		}
	}

	// 2) Sem botão? Faz scroll incremental forte até o rodapé
	docH, _ := wd.ExecuteScript("return document.body.scrollHeight;", nil)
	startH := int64(0)
	if v, ok := docH.(float64); ok {
		startH = int64(v)
	}
	for i := 0; i < 6; i++ { // ~6 passos de scroll
		_, _ = wd.ExecuteScript("window.scrollBy(0, window.innerHeight*0.9);", nil)
		time.Sleep(600 * time.Millisecond)
	}
	// mais uma descida ao fundo
	_, _ = wd.ExecuteScript("window.scrollTo(0, document.body.scrollHeight);", nil)
	time.Sleep(1200 * time.Millisecond)

	// se a altura do documento mudou, aumentou conteúdo -> retorno “true” (houve ação)
	newHraw, _ := wd.ExecuteScript("return document.body.scrollHeight;", nil)
	newH := startH
	if v, ok := newHraw.(float64); ok {
		newH = int64(v)
	}
	return newH > startH
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
	clicks := flag.Int("c", 10000, "Max scroll loads to fetch more results (default: 10)")
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
