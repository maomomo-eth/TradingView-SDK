# TradingView SDK

这是一个按 TradingView 前端接口整理的多语言 SDK 仓库，目前包含：

- PHP 8+ SDK：已迁移到独立仓库 [maomomo-eth/TradingView-PHP-SDK](https://github.com/maomomo-eth/TradingView-PHP-SDK)，Composer 包名 `maomomo-eth/tradingview-php-sdk`。
- `go/`：Go SDK。
- `python/`：Python 3.10+ SDK。

SDK 覆盖的主要接口：

- `https://symbol-search.tradingview.com/symbol_search/v3/`：搜索 TradingView 标的，适合拿 `full_name`。
- `https://scanner.tradingview.com/symbol`：查询单标的报价、详情、表现、技术评级字段。
- `https://scanner.tradingview.com/{market}/scan`：批量查询或筛选 Scanner 数据。
- `https://scanner.tradingview.com/forex/scan?label-product=related-symbols`：查询外汇相关标的。

这些接口属于 TradingView 前端使用的非正式接口，不是 TradingView 官方承诺稳定的公开行情 API。请自用或低频使用，并做好缓存、重试、限速、字段缺失兼容。不要在代码里写入 `sessionid`、`sessionid_sign` 等登录 Cookie。

## 目录

```text
.
├── go/       Go SDK
└── python/   Python SDK
```

## 快速示例

### PHP

```powershell
composer require maomomo-eth/tradingview-php-sdk
```

```php
$client = new TradingView\TradingViewClient();
$results = $client->searchSymbols('BTC', [
    'search_type' => 'crypto',
    'exchange' => 'BINANCE',
    'lang' => 'zh',
]);
```

### Go

```powershell
cd go
go run .\examples\basic
```

```go
client, _ := tradingview.NewClient(nil)
results, err := client.SearchSymbols(ctx, "BTC", &tradingview.SearchSymbolsOptions{
	SearchType: "crypto",
	Exchange:   "BINANCE",
	Lang:       "zh",
})
```

### Python

```powershell
cd python
python .\examples\basic.py
```

```python
client = TradingViewClient()
results = client.search_symbols(
    "BTC",
    SearchSymbolsOptions(search_type="crypto", exchange="BINANCE", lang="zh"),
)
```

## 使用建议

- 搜索标的时优先保存 TradingView 返回的 `full_name`，例如 `BINANCE:BTCUSDT`、`NASDAQ:AAPL`、`OANDA:EURUSD`。
- 报价查询时使用同一个 TradingView symbol 体系，避免第三方 ticker 和 TradingView 数据源不一致。
- `symbol_search/v3` 是搜索接口，不是全量列表下载接口；初始化列表时建议用候选标的分批搜索并缓存。
- 股票、加密货币、外汇可共用 `searchSymbols/search_symbols/SearchSymbols`，通过 `search_type` 和 `exchange` 控制范围。
- 网络不稳定或直连超时时，可通过 `TRADINGVIEW_PROXY` 或各语言 SDK 的 `proxy` 配置传入代理。

## 文档

- [PHP SDK](https://github.com/maomomo-eth/TradingView-PHP-SDK)
- [Go SDK](go/README.md)
- [Python SDK](python/README.md)
