# Ducker 🦆

Ducker is a powerful command-line tool for performing advanced DuckDuckGo dorking using Selenium in Go. It allows you to extract search result links with custom queries and navigational options.

## Prerequisites

- Go (1.16+)
- ChromeDriver installed and accessible in your PATH
- Google Chrome browser

## Installation

### Using go install

```bash
go install github.com/gilsgil/ducker@latest
```

### From Source

```bash
git clone https://github.com/gilsgil/ducker.git
cd ducker
go build
```

## Usage

```bash
ducker -q "search query" [-c number_of_clicks] [-v]
```

### Flags

- `-q`: Search query (required)
- `-c`: Maximum number of "More results" clicks (default: 10)
- `-v`: Enable verbose output

## Examples

### Basic Dork Queries

1. Search for PDF files on a specific site:
```bash
ducker -q "filetype:pdf site:example.com"
```

2. Find sensitive data mentions:
```bash
ducker -q "intext:\"sensitive data\""
```

3. Advanced search with verbose output:
```bash
ducker -q "site:gov intitle:confidential" -c 5 -v
```

### DuckDuckGo Dork Examples

1. Find exposed directories:
```bash
ducker -q "intitle:\"index of\" site:example.com"
```

2. Find specific file types:
```bash
ducker -q "filetype:xls password site:org"
```

3. Search for exposed configuration files:
```bash
ducker -q "filetype:ini OR filetype:conf site:example.net"
```

## Security and Legal Notice

🚨 **Important**: This tool is for educational and authorized testing purposes only. Always ensure you have explicit permission before conducting searches or accessing resources. Unauthorized scanning or access may be illegal.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request on the GitHub repository.

## License

[Specify your license here, e.g., MIT License]

## Disclaimer

Use this tool responsibly and ethically. The authors are not responsible for misuse.
