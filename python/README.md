# TradingView Scanner Python SDK

这是 `php` 版本 SDK 的 Python 实现，使用标准库 `urllib`，不依赖第三方包。它封装 TradingView 前端使用的非正式接口：

- `symbol_search/v3`：按关键词搜索 TradingView 标的。
- `/symbol`：查询单标的报价、详情、表现、技术评级字段。
- `/{market}/scan`：按标的列表或筛选条件批量查询 Scanner 数据。
- `/forex/scan?label-product=related-symbols`：查询外汇相关标的。

这些接口不是 TradingView 官方承诺稳定的公开行情 API。生产环境请做缓存、限速、重试和字段缺失兼容，不要把登录 Cookie 写进代码。

## 快速开始

```python
from tradingview import SearchSymbolsOptions, TradingViewClient

client = TradingViewClient()

results = client.search_symbols(
    "BTC",
    SearchSymbolsOptions(search_type="crypto", exchange="BINANCE", lang="zh"),
)
print(results)
```

运行示例：

```powershell
cd python
python .\examples\basic.py
```

代理：

```powershell
$env:TRADINGVIEW_PROXY = "http://127.0.0.1:7890"
python .\examples\basic.py

python .\examples\basic.py socks5://127.0.0.1:1080
```

## 常用方法

```python
client.search_symbols("BTC", {"search_type": "crypto", "exchange": "BINANCE"})

client.get_symbol_quote("BINANCE:BTCUSDT")
client.get_symbol_details("NASDAQ:AAPL")
client.get_symbol_performance("NASDAQ:AAPL")
client.get_symbol_technicals("NASDAQ:AAPL")

scan = client.scan_symbols("crypto", ["BINANCE:BTCUSDT"], ["name", "close"])
rows = client.map_scan_rows(scan, ["name", "close"])
```

`search_symbols` 返回结果中的 `full_name` 通常可作为后续报价查询的 symbol，例如 `BINANCE:BTCUSDT`、`NASDAQ:AAPL`、`OANDA:EURUSD`。
