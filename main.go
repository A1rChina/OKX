package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Ticker 表示 OKX API 返回的单个 ticker 数据结构
type Ticker struct {
	InstID    string `json:"instId"`
	Last      string `json:"last"`
	VolCcy24h string `json:"volCcy24h"`
}

// APIResponse 表示 API 返回的数据结构
type APIResponse struct {
	Data []Ticker `json:"data"`
}

// TickerRow 用于存储筛选后的数据，方便后续排序和展示
type TickerRow struct {
	InstID      string
	LastPrice   float64
	DailyVolume float64
}

const OKXAPIURL = "https://www.okx.com/api/v5/market/tickers?instType=SWAP"

// transformInstID 将 Instrument ID 转换为期望格式：
// 如 "BTC-USDT-SWAP" 转换为 "BTCUSDT"（保留前两部分，并去掉 "-" 连接符）
func transformInstID(instID string) string {
	parts := strings.Split(instID, "-")
	if len(parts) >= 2 {
		return parts[0] + parts[1]
	}
	return instID
}

// formatVolume 根据数值大小返回带单位的字符串表示形式。
// 如果数值大于等于 1e9（十亿），则转换为 B 单位；
// 如果大于等于 1e6（百万），则转换为 M 单位；
// 否则直接返回原始数值，均保留 3 位小数。
func formatVolume(volume float64) string {
	if volume >= 1e9 {
		return fmt.Sprintf("%.3fB", volume/1e9)
	} else if volume >= 1e6 {
		return fmt.Sprintf("%.3fM", volume/1e6)
	}
	return fmt.Sprintf("%.3f", volume)
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

	// 筛选 USDT 交易对，并计算 24h 交易额（单位 USDT）
	// 只保留日交易额大于等于 3000 万 USDT 的数据
	var rows []TickerRow
	for _, ticker := range apiResponse.Data {
		// 只处理 Instrument ID 中包含 "USDT" 的交易对
		if !strings.Contains(ticker.InstID, "USDT") {
			continue
		}

		// 将字符串转换为浮点数
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
			rows = append(rows, TickerRow{
				InstID:      transformedID,
				LastPrice:   last,
				DailyVolume: dailyVolume,
			})
		}
	}

	// 按照 24h 交易额从大到小排序
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].DailyVolume > rows[j].DailyVolume
	})

	// 生成 Markdown 格式的表格文档
	output := "# USDT Perpetual Contracts with Daily Volume > $30 Million\n\n"
	output += "| Rank | Instrument ID | Last Price (USDT) | 24h Volume (USDT) |\n"
	output += "|------|---------------|-------------------|-------------------|\n"
	for i, row := range rows {
		// 24h Volume 按照要求保留 3 位小数并带单位
		volumeStr := formatVolume(row.DailyVolume)
		// 此处 Last Price 同样保留 3 位小数，如需保留 2 位可将 %.3f 改为 %.2f
		output += fmt.Sprintf("| %d | %s | %.3f | %s |\n", i+1, row.InstID, row.LastPrice, volumeStr)
	}

	// 写入 README.md 文件
	if err := ioutil.WriteFile("README.md", []byte(output), 0644); err != nil {
		fmt.Printf("Error writing to README.md: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("README.md updated successfully.")
}
