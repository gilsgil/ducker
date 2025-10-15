package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/tebeka/selenium"
	"github.com/tebeka/selenium/chrome"
)

const (
	chromeDriverPort = 9515
)

// ---------- Utilidades ----------

func humanPause(min, max time.Duration) {
	d := min + time.Duration(rand.Int63n(int64(max-min)))
	time.Sleep(d)
}

func runesPrefix(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n])
}

// ---------- Núcleo ----------

func duckduckgoDorking(query, period string, verbose bool, stallLimit int, lang string) {
	rand.Seed(time.Now().UnixNano())

	// Sobe ChromeDriver
	svc, err := selenium.NewChromeDriverService("chromedriver", chromeDriverPort)
	if err != nil {
		log.Fatalf("Erro iniciando ChromeDriver: %v", err)
	}
	defer svc.Stop()

	// Caps do Chrome
	caps := selenium.Capabilities{"browserName": "chrome"}
	chromeCaps := chrome.Capabilities{
		Args: []string{
			"--headless=new",
			"--no-sandbox",
			"--disable-dev-shm-usage",
			"--disable-gpu",
			"--window-size=1280,1200",
			"--lang=" + lang,
			"--accept-lang=" + lang,
			"--disable-blink-features=AutomationControlled",
			`--user-agent=Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36`,
		},
		ExcludeSwitches: []string{"enable-automation"},
		Prefs: map[string]interface{}{
			"intl.accept_languages": lang,
		},
	}
	caps.AddChrome(chromeCaps)

	wd, err := selenium.NewRemote(caps, fmt.Sprintf("http://localhost:%d/wd/hub", chromeDriverPort))
	if err != nil {
		log.Fatalf("Erro conectando no WebDriver: %v", err)
	}
	defer wd.Quit()

	// Inocula navigator.webdriver = false
	_, _ = wd.ExecuteScript(`Object.defineProperty(navigator, 'webdriver', {get: () => undefined});`, nil)

	// Monta URL
	escaped := url.QueryEscape(query)
	searchURL := fmt.Sprintf("https://duckduckgo.com/?q=%s&t=h_&ia=web", escaped)
	if period != "" {
		searchURL += "&df=" + period // d,w,m,y
	}
	if verbose {
		fmt.Println("[info] abrindo:", searchURL)
	}
	if err := wd.Get(searchURL); err != nil {
		log.Fatalf("Erro abrindo a página: %v", err)
	}

	// Possível banner de consentimento/captcha
	handleConsentAndCaptcha(wd, verbose)

	// Seletor dos resultados (DDG muda às vezes; mantemos dois caminhos)
	// - a[data-testid='result-title-a'] é o mais estável hoje
	// - fallback em h2 > a
	resultSelectors := []string{
		"a[data-testid='result-title-a']",
		"h2 > a",
	}

	// Espera primeiros resultados
	if err := waitAnyVisible(wd, resultSelectors, 20*time.Second); err != nil && verbose {
		title, _ := wd.Title()
		html, _ := wd.PageSource()
		fmt.Println("[warn] Nenhum resultado visível após timeout. Title:", title)
		fmt.Println("[debug] HTML(500):", runesPrefix(html, 500))
	}

	seen := make(map[string]struct{})
	prevTotal := 0
	stallCount := 0

	for {
		// Coleta & imprime novos links já presentes no DOM
		totalElems := 0
		for _, sel := range resultSelectors {
			els, _ := wd.FindElements(selenium.ByCSSSelector, sel)
			totalElems += collectAndPrint(wd, els, seen)
		}

		if totalElems > prevTotal {
			if verbose {
				fmt.Printf("[info] carregados %d elementos (antes %d)\n", totalElems, prevTotal)
			}
			prevTotal = totalElems
			stallCount = 0
		} else {
			stallCount++
			if verbose {
				fmt.Printf("[info] sem crescimento (%d/%d)\n", stallCount, stallLimit)
			}
		}

		// Se ficou “estagnado” muitas vezes, tentamos reforçar carregamento (clique + scroll forte)
		if stallCount > 0 {
			clicked := tryClickMore(wd, verbose)
			if clicked {
				humanPause(800*time.Millisecond, 1500*time.Millisecond)
			} else {
				// Scroll forte
				strongScroll(wd)
				humanPause(1200*time.Millisecond, 2200*time.Millisecond)
			}

			// Recheca crescimento após ação
			newCount := 0
			for _, sel := range resultSelectors {
				els, _ := wd.FindElements(selenium.ByCSSSelector, sel)
				newCount += len(els)
			}
			if newCount > prevTotal {
				if verbose {
					fmt.Printf("[info] +conteúdo após ação: %d (antes %d)\n", newCount, prevTotal)
				}
				prevTotal = newCount
				stallCount = 0
				continue
			}
		}

		// Se batemos o limite de estagnação, acabou
		if stallCount >= stallLimit {
			if verbose {
				fmt.Println("[info] nenhum crescimento após várias tentativas; fim dos resultados.")
			}
			break
		}

		// Toque leve entre ciclos
		humanPause(600*time.Millisecond, 1400*time.Millisecond)
	}
}

func collectAndPrint(wd selenium.WebDriver, els []selenium.WebElement, seen map[string]struct{}) int {
	countBefore := len(seen)
	for _, e := range els {
		href, _ := e.GetAttribute("href")
		if href == "" {
			// alguns títulos são <a> sem href e o link verdadeiro fica no parent – ignoramos
			continue
		}
		// normaliza
		h := strings.TrimSpace(href)
		if h == "" {
			continue
		}
		if _, ok := seen[h]; !ok {
			seen[h] = struct{}{}
			fmt.Println(h)
		}
	}
	return len(seen) - countBefore
}

// tenta clicar no botão "More results"
func tryClickMore(wd selenium.WebDriver, verbose bool) bool {
	btnSelectors := []string{
		"#more-results",
		"button.result--more__btn",
		"a.result--more__btn",
		"button[aria-label*='More']",
		"a[aria-label*='More']",
	}

	for _, q := range btnSelectors {
		btns, _ := wd.FindElements(selenium.ByCSSSelector, q)
		if len(btns) == 0 {
			continue
		}
		btn := btns[0]
		// traz ao viewport
		_, _ = wd.ExecuteScript("arguments[0].scrollIntoView({block:'center'});", []interface{}{btn})
		humanPause(200*time.Millisecond, 500*time.Millisecond)

		// tenta click normal primeiro
		if err := btn.Click(); err != nil {
			// força via JS
			_, _ = wd.ExecuteScript("arguments[0].click();", []interface{}{btn})
		}
		if verbose {
			fmt.Println("[info] clique no botão de mais resultados:", q)
		}
		return true
	}
	return false
}

// scroll forte ao fundo + pequenas etapas
func strongScroll(wd selenium.WebDriver) {
	// passos incrementais
	for i := 0; i < 8; i++ {
		_, _ = wd.ExecuteScript("window.scrollBy(0, Math.floor(window.innerHeight*0.9));", nil)
		time.Sleep(250 * time.Millisecond)
	}
	// vai ao fundo
	_, _ = wd.ExecuteScript("window.scrollTo(0, document.body.scrollHeight);", nil)
}

// espera qualquer seletor aparecer
func waitAnyVisible(wd selenium.WebDriver, selectors []string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, sel := range selectors {
			els, _ := wd.FindElements(selenium.ByCSSSelector, sel)
			if len(els) > 0 {
				return nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("timeout esperando resultados")
}

// tenta lidar com consentimento/captcha básicos
func handleConsentAndCaptcha(wd selenium.WebDriver, verbose bool) {
	// consentimentos comuns
	consentSelectors := []string{
		"form[action*='consent'] button[type='submit']",
		"button#consent-accept-button",
		"button[mode='primary']",
		"button[aria-label*='Accept']",
	}
	for _, q := range consentSelectors {
		btns, _ := wd.FindElements(selenium.ByCSSSelector, q)
		if len(btns) > 0 {
			_, _ = wd.ExecuteScript("arguments[0].scrollIntoView({block:'center'});", []interface{}{btns[0]})
			_ = btns[0].Click()
			if verbose {
				fmt.Println("[info] clique em consentimento:", q)
			}
			humanPause(500*time.Millisecond, 1200*time.Millisecond)
			break
		}
	}

	// detecção de captcha (heurística)
	html, _ := wd.PageSource()
	if strings.Contains(strings.ToLower(html), "unusual traffic") ||
		strings.Contains(strings.ToLower(html), "captcha") {
		fmt.Fprintln(os.Stderr, "[warn] página indicou captcha/bloqueio de tráfego incomum.")
	}
}

// ---------- Main ----------

func main() {
	query := flag.String("q", "", "Query para buscar no DuckDuckGo (obrigatório)")
	day := flag.Bool("day", false, "Filtrar por último dia")
	week := flag.Bool("week", false, "Filtrar por última semana")
	month := flag.Bool("month", false, "Filtrar por último mês")
	year := flag.Bool("year", false, "Filtrar por último ano")
	verbose := flag.Bool("v", false, "Logs extras")
	stallLimit := flag.Int("stall", 3, "Número de tentativas seguidas sem crescimento antes de encerrar")
	lang := flag.String("lang", "pt-BR", "Idioma/Accept-Language para o navegador")

	flag.Parse()

	if strings.TrimSpace(*query) == "" {
		fmt.Println("Use -q para informar a query. Ex: -q \"site:exemplo.com filetype:pdf\"")
		flag.Usage()
		os.Exit(1)
	}

	var period string
	switch {
	case *day:
		period = "d"
	case *week:
		period = "w"
	case *month:
		period = "m"
	case *year:
		period = "y"
	}

	duckduckgoDorking(*query, period, *verbose, *stallLimit, *lang)
}
