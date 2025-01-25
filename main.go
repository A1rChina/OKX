package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
)

// Ticker represents the structure of a single ticker from OKX API
type Ticker struct {
	InstID     string  `json:"instId"`
	Last       string  `json:"last"`
	VolCcy24h  string  `json:"volCcy24h"`
}

// APIResponse represents the structure of the API response
type APIResponse struct {
	Data []Ticker `json:"data"`
}

const (
	OKXAPIURL = "https://www.okx.com/api/v5/market/tickers?instType=SWAP"
)

func main() {
	// Fetch data from OKX API
	response, err := http.Get(OKXAPIURL)
	if err != nil {
		fmt.Printf("Error fetching data: %v\n", err)
		os.Exit(1)
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		fmt.Printf("Error reading response body: %v\n", err)
		os.Exit(1)
	}

	// Parse JSON response
	var apiResponse APIResponse
	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		fmt.Printf("Error parsing JSON: %v\n", err)
		os.Exit(1)
	}

	// Filter and calculate daily trading volume in USDT
	filteredTickers := [][]string{{"Instrument ID", "Last Price (USDT)", "24h Volume (USDT)"}}
	for _, ticker := range apiResponse.Data {
		last, err := strconv.ParseFloat(ticker.Last, 64)
		if err != nil {
			continue
		}
		volCcy24h, err := strconv.ParseFloat(ticker.VolCcy24h, 64)
		if err != nil {
			continue
		}
		dailyVolume := last * volCcy24h
		if dailyVolume >= 30000000 {
			filteredTickers = append(filteredTickers, []string{
				ticker.InstID, fmt.Sprintf("%.2f", last), fmt.Sprintf("%.2f", dailyVolume),
			})
		}
	}

	// Generate Markdown table
	output := "# USDT Perpetual Contracts with Daily Volume > $30 Million\n\n"
	output += "| Instrument ID | Last Price (USDT) | 24h Volume (USDT) |\n"
	output += "|---------------|-------------------|-------------------|\n"
	for _, row := range filteredTickers[1:] {
		output += fmt.Sprintf("| %s | %s | %s |\n", row[0], row[1], row[2])
	}

	// Write to README.md
	if err := ioutil.WriteFile("README.md", []byte(output), 0644); err != nil {
		fmt.Printf("Error writing to README.md: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("README.md updated successfully.")
}
