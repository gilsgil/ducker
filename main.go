package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/tebeka/selenium"
	"github.com/tebeka/selenium/chrome"
)

func duckduckgoDorking(query string, maxClicks int, verbose bool) {
	const port = 9515

	// Inicializa o serviço do ChromeDriver (certifique-se de que o "chromedriver" esteja no PATH)
	service, err := selenium.NewChromeDriverService("chromedriver", port)
	if err != nil {
		log.Fatalf("Erro ao iniciar o serviço do ChromeDriver: %v", err)
	}
	defer service.Stop()

	// Configura as capacidades para o Chrome
	caps := selenium.Capabilities{"browserName": "chrome"}
	chromeCaps := chrome.Capabilities{
		Args: []string{
			"--headless", // Remove esta linha se quiser ver o navegador em ação
			"--no-sandbox",
			"--disable-dev-shm-usage",
		},
	}
	caps.AddChrome(chromeCaps)

	// Conecta ao WebDriver
	wd, err := selenium.NewRemote(caps, fmt.Sprintf("http://localhost:%d/wd/hub", port))
	if err != nil {
		log.Fatalf("Erro ao conectar ao WebDriver: %v", err)
	}
	defer wd.Quit()

	// Acessa a URL com a consulta
	url := fmt.Sprintf("https://duckduckgo.com/?q=%s", query)
	if err := wd.Get(url); err != nil {
		log.Fatalf("Erro ao carregar a página: %v", err)
	}

	// Aguarda o carregamento dos resultados iniciais
	time.Sleep(5 * time.Second)

	allLinks := make(map[string]bool)
	clickCount := 0

	// Loop para coletar links e clicar em "Mais resultados"
	for clickCount < maxClicks {
		// Encontra todos os elementos dos resultados com o seletor CSS especificado
		elems, err := wd.FindElements(selenium.ByCSSSelector, "a[data-testid='result-title-a']")
		if err != nil {
			if verbose {
				fmt.Println("Erro ao encontrar os elementos dos resultados:", err)
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

		// Tenta localizar o botão "Mais resultados" pelo ID
		moreResults, err := wd.FindElement(selenium.ByID, "more-results")
		if err != nil {
			if verbose {
				fmt.Println("Botão 'Mais resultados' não encontrado ou não está mais disponível.")
			}
			break
		}

		// Clica no botão usando JavaScript para garantir a execução
		_, err = wd.ExecuteScript("arguments[0].click();", []interface{}{moreResults})
		if err != nil {
			if verbose {
				fmt.Println("Erro ao clicar no botão 'Mais resultados':", err)
			}
			break
		}
		if verbose {
			fmt.Println("Clicando no botão 'Mais resultados' para carregar mais resultados...")
		}
		clickCount++
		// Pausa para permitir o carregamento dos novos resultados
		time.Sleep(4 * time.Second)
	}
}

func main() {
	// Define os parâmetros de linha de comando
	query := flag.String("q", "", "Consulta para buscar no DuckDuckGo")
	clicks := flag.Int("c", 10, "Número máximo de cliques para carregar mais resultados (padrão: 10)")
	verbose := flag.Bool("v", false, "Mostra mensagens adicionais de status")

	flag.Parse()

	if *query == "" {
		fmt.Println("É necessário fornecer uma consulta com -q ou --query")
		flag.Usage()
		os.Exit(1)
	}

	duckduckgoDorking(*query, *clicks, *verbose)
}
