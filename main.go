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

// APIResponse 表示 /market/tickers 返回的数据
type APIResponse struct {
	Data []Ticker `json:"data"`
}

// CandleAPIResponse 表示 /market/candles 返回的数据
// OKX 的 data 是一个二维数组，每一项是 [ts, o, h, l, c, ...]
type CandleAPIResponse struct {
	Data [][]string `json:"data"`
}

// TickerRow 用于存储筛选后的数据
type TickerRow struct {
	InstID         string  // 例如 BTCUSDT（展示用）
	RawInstID      string  // 例如 BTC-USDT-SWAP（请求 K 线用）
	DailyVolume    float64 // 24h 成交额（USDT）
	Volatility4hPc float64 // 最近 4 小时波动幅度百分比
}

const (
	OKXTickersURL = "https://www.okx.com/api/v5/market/tickers?instType=SWAP"
	OKXCandlesURL = "https://www.okx.com/api/v5/market/candles"
)

// transformInstID 将 Instrument ID 转换为展示格式：
// 如 "BTC-USDT-SWAP" 转换为 "BTCUSDT"
func transformInstID(instID string) string {
	parts := strings.Split(instID, "-")
	if len(parts) >= 2 {
		return parts[0] + parts[1]
	}
	return instID
}

// formatVolume 根据数值大小返回带单位的字符串表示形式。
// >= 1e9 -> B，>= 1e6 -> M，否则原数值，均保留 3 位小数。
func formatVolume(volume float64) string {
	if volume >= 1e9 {
		return fmt.Sprintf("%.3fB", volume/1e9)
	} else if volume >= 1e6 {
		return fmt.Sprintf("%.3fM", volume/1e6)
	}
	return fmt.Sprintf("%.3f", volume)
}

// fetch4hVolatility 从 OKX 拉取最近一根 4 小时 K 线，并计算波动幅度百分比：
// (high - low) / open * 100
func fetch4hVolatility(rawInstID string) (float64, error) {
	url := fmt.Sprintf("%s?instId=%s&bar=4H&limit=1", OKXCandlesURL, rawInstID)

	resp, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("error fetching 4H candles for %s: %w", rawInstID, err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("error reading 4H candles body for %s: %w", rawInstID, err)
	}

	var candleResp CandleAPIResponse
	if err := json.Unmarshal(body, &candleResp); err != nil {
		return 0, fmt.Errorf("error parsing 4H candles JSON for %s: %w", rawInstID, err)
	}

	if len(candleResp.Data) == 0 {
		return 0, fmt.Errorf("no 4H candle data for %s", rawInstID)
	}

	// data[0] = [ts, o, h, l, c, ...]
	candle := candleResp.Data[0]
	if len(candle) < 4 {
		return 0, fmt.Errorf("invalid 4H candle data for %s", rawInstID)
	}

	open, err := strconv.ParseFloat(candle[1], 64)
	if err != nil {
		return 0, fmt.Errorf("error parsing open for %s: %w", rawInstID, err)
	}
	high, err := strconv.ParseFloat(candle[2], 64)
	if err != nil {
		return 0, fmt.Errorf("error parsing high for %s: %w", rawInstID, err)
	}
	low, err := strconv.ParseFloat(candle[3], 64)
	if err != nil {
		return 0, fmt.Errorf("error parsing low for %s: %w", rawInstID, err)
	}

	if open <= 0 {
		return 0, fmt.Errorf("open price <= 0 for %s", rawInstID)
	}

	volatility := (high - low) / open * 100
	return volatility, nil
}

func main() {
	// 1. 从 OKX API 获取 SWAP tickers
	response, err := http.Get(OKXTickersURL)
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

	// 2. 解析 JSON 数据
	var apiResponse APIResponse
	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		fmt.Printf("Error parsing JSON: %v\n", err)
		os.Exit(1)
	}

	// 3. 筛选 USDT 交易对，计算 24h 成交额（单位 USDT），只保留 >= 5000 万 USDT
	var rows []TickerRow
	for _, ticker := range apiResponse.Data {
		if !strings.Contains(ticker.InstID, "USDT") {
			continue
		}

		last, err := strconv.ParseFloat(ticker.Last, 64)
		if err != nil {
			continue
		}
		volCcy24h, err := strconv.ParseFloat(ticker.VolCcy24h, 64)
		if err != nil {
			continue
		}
		dailyVolume := last * volCcy24h

		if dailyVolume >= 50000000 {
			transformedID := transformInstID(ticker.InstID)
			rows = append(rows, TickerRow{
				InstID:      transformedID,
				RawInstID:   ticker.InstID,
				DailyVolume: dailyVolume,
			})
		}
	}

	// 4. 对入选标的，逐个拉取最近 4 小时波动幅度百分比
	for i := range rows {
		v, err := fetch4hVolatility(rows[i].RawInstID)
		if err != nil {
			// 出错时保留 0，打印日志但不中断整个程序
			fmt.Printf("Warning: %v\n", err)
			rows[i].Volatility4hPc = 0
			continue
		}
		rows[i].Volatility4hPc = v
	}

	// 5. 按 24h 成交额从大到小排序
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].DailyVolume > rows[j].DailyVolume
	})

	// 6. 生成 Markdown 文件
	// 标题：中文、简洁
	output := "# USDT 永续合约成交额排行\n\n"

	// 使用你现在的简洁表头，并新增一列 4 小时波动幅度
	output += "| 排名 | 合约 | 24h 成交额 | 4h 波动幅度 |\n"
	output += "|------|------|------------|-------------|\n"

	for i, row := range rows {
		volumeStr := formatVolume(row.DailyVolume)
		output += fmt.Sprintf(
			"| %d | %s | %s | %.2f%% |\n",
			i+1,
			row.InstID,
			volumeStr,
			row.Volatility4hPc,
		)
	}

	// 7. 写入 README.md
	if err := ioutil.WriteFile("README.md", []byte(output), 0644); err != nil {
		fmt.Printf("Error writing to README.md: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("README.md updated successfully.")
}
