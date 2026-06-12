package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	tradingview "tradingview"
)

func dump(title string, value any) {
	fmt.Println("== " + title + " ==")
	bytes, _ := json.MarshalIndent(value, "", "  ")
	fmt.Println(string(bytes))
}

func main() {
	ctx := context.Background()
	proxy := ""
	if len(os.Args) > 1 {
		proxy = os.Args[1]
	} else {
		proxy = os.Getenv("TRADINGVIEW_PROXY")
	}

	client, err := tradingview.NewClient(&tradingview.ClientOptions{
		Timeout:        30 * time.Second,
		ConnectTimeout: 15 * time.Second,
		Proxy:          proxy,
	})
	if err != nil {
		panic(err)
	}

	quote, err := client.GetSymbolQuote(ctx, "IBKR:CNHHKD", nil, nil)
	if err != nil {
		fail(err)
	}
	dump("quote", quote)

	search, err := client.SearchSymbols(ctx, "BTC", &tradingview.SearchSymbolsOptions{
		SearchType: "crypto",
		Exchange:   "BINANCE",
		Lang:       "zh",
	})
	if err != nil {
		fail(err)
	}
	dump("search", search)

	stockColumns := []string{"name", "description", "close", "change", "change_abs", "volume", "market_cap_basic"}
	stockScan, err := client.ScanSymbols(ctx, "america", []string{"NASDAQ:AAPL", "NASDAQ:MSFT", "NASDAQ:TSLA"}, stockColumns, &tradingview.ScanSymbolsOptions{
		Lang: "zh",
	})
	if err != nil {
		fail(err)
	}
	stockRows, err := tradingview.MapScanRows(stockScan, stockColumns)
	if err != nil {
		fail(err)
	}
	dump("stocks", stockRows)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "请求失败："+err.Error())
	fmt.Fprintln(os.Stderr, `如果直连 TradingView 超时，请配置代理，例如：$env:TRADINGVIEW_PROXY="http://127.0.0.1:7890"; go run .\examples\basic`)
	os.Exit(1)
}
