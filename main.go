package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// Ticker represents the structure of a single ticker from OKX API.
type Ticker struct {
	InstID    string `json:"instId"`
	Last      string `json:"last"`
	VolCcy24h string `json:"volCcy24h"`
}

// APIResponse represents the structure of the API response.
type APIResponse struct {
	Data []Ticker `json:"data"`
}

const (
	OKXAPIURL = "https://www.okx.com/api/v5/market/tickers?instType=SWAP"
)

// transformInstID 将 Instrument ID 转换为期望格式：
// 如 "BTC-USDT-SWAP" 转换为 "BTCUSDT"（保留前两部分，并去掉 "-" 连接符）
func transformInstID(instID string) string {
	parts := strings.Split(instID, "-")
	if len(parts) >= 2 {
		return parts[0] + parts[1]
	}
	return instID
}

func main() {
	// 从 OKX API 获取数据
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

	// 解析 JSON 数据
	var apiResponse APIResponse
	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		fmt.Printf("Error parsing JSON: %v\n", err)
		os.Exit(1)
	}

	// 筛选 USDT 交易对，并计算 24h 交易额（单位 USDT），只保留日交易额大于等于 3000 万的交易对
	filteredTickers := [][]string{{"Instrument ID", "Last Price (USDT)", "24h Volume (USDT)"}}
	for _, ticker := range apiResponse.Data {
		// 只处理 USDT 交易对（Instrument ID 中必须包含 "USDT"）
		if !strings.Contains(ticker.InstID, "USDT") {
			continue
		}

		// 转换数字字符串为浮点数
		last, err := strconv.ParseFloat(ticker.Last, 64)
		if err != nil {
			continue
		}
		volCcy24h, err := strconv.ParseFloat(ticker.VolCcy24h, 64)
		if err != nil {
			continue
		}
		dailyVolume := last * volCcy24h

		// 只保留日交易额大于等于 3000 万 USDT 的数据
		if dailyVolume >= 30000000 {
			transformedID := transformInstID(ticker.InstID)
			filteredTickers = append(filteredTickers, []string{
				transformedID,
				fmt.Sprintf("%.2f", last),
				fmt.Sprintf("%.2f", dailyVolume),
			})
		}
	}

	// 生成 Markdown 格式的表格文档
	output := "# USDT Perpetual Contracts with Daily Volume > $30 Million\n\n"
	output += "| Instrument ID | Last Price (USDT) | 24h Volume (USDT) |\n"
	output += "|---------------|-------------------|-------------------|\n"
	for _, row := range filteredTickers[1:] {
		output += fmt.Sprintf("| %s | %s | %s |\n", row[0], row[1], row[2])
	}

	// 写入 README.md 文件
	if err := ioutil.WriteFile("README.md", []byte(output), 0644); err != nil {
		fmt.Printf("Error writing to README.md: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("README.md updated successfully.")
}
