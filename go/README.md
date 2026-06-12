# TradingView Scanner Go SDK

这是 `php` 版本 SDK 的 Go 实现，封装 TradingView 前端使用的非正式接口：

- `symbol_search/v3`：按关键词搜索 TradingView 标的。
- `/symbol`：查询单标的报价、详情、表现、技术评级字段。
- `/{market}/scan`：按标的列表或筛选条件批量查询 Scanner 数据。
- `/forex/scan?label-product=related-symbols`：查询外汇相关标的。

这些接口不是 TradingView 官方承诺稳定的公开行情 API。生产环境请做缓存、限速、重试和字段缺失兼容，不要把登录 Cookie 写进代码。

## 快速开始

```go
package main

import (
	"context"
	"fmt"

	tradingview "tradingview"
)

func main() {
	client, _ := tradingview.NewClient(nil)

	results, err := client.SearchSymbols(context.Background(), "BTC", &tradingview.SearchSymbolsOptions{
		SearchType: "crypto",
		Exchange:   "BINANCE",
		Lang:       "zh",
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(results)
}
```

运行示例：

```powershell
cd go
go run .\examples\basic
```

代理：

```powershell
$env:TRADINGVIEW_PROXY = "http://127.0.0.1:7890"
go run .\examples\basic

go run .\examples\basic socks5://127.0.0.1:1080
```

## 常用方法

```go
client.SearchSymbols(ctx, "BTC", &tradingview.SearchSymbolsOptions{
	SearchType: "crypto",
	Exchange:   "BINANCE",
	Lang:       "zh",
})

client.GetSymbolQuote(ctx, "BINANCE:BTCUSDT", nil, nil)
client.GetSymbolDetails(ctx, "NASDAQ:AAPL", nil, nil)
client.GetSymbolPerformance(ctx, "NASDAQ:AAPL", nil, nil)
client.GetSymbolTechnicals(ctx, "NASDAQ:AAPL", nil, nil)

scan, _ := client.ScanSymbols(ctx, "crypto", []string{"BINANCE:BTCUSDT"}, tradingview.QuoteFields, nil)
rows, _ := tradingview.MapScanRows(scan, tradingview.QuoteFields)
```

`SearchSymbols` 返回结果中的 `full_name` 通常可作为后续报价查询的 symbol，例如 `BINANCE:BTCUSDT`、`NASDAQ:AAPL`、`OANDA:EURUSD`。
