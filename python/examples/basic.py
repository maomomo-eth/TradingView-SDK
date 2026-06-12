import json
import os
import sys

from tradingview import QUOTE_FIELDS, ScanSymbolsOptions, SearchSymbolsOptions, TradingViewClient, TradingViewError


def dump(title, value):
    print(f"== {title} ==")
    print(json.dumps(value, ensure_ascii=False, indent=2))


def main():
    proxy = sys.argv[1] if len(sys.argv) > 1 else os.getenv("TRADINGVIEW_PROXY")
    client = TradingViewClient(
        {
            "timeout": int(os.getenv("TRADINGVIEW_TIMEOUT", "30")),
            "connect_timeout": int(os.getenv("TRADINGVIEW_CONNECT_TIMEOUT", "15")),
            "proxy": proxy,
        }
    )

    quote = client.get_symbol_quote("IBKR:CNHHKD")
    dump("quote", quote)

    search = client.search_symbols(
        "BTC",
        SearchSymbolsOptions(search_type="crypto", exchange="BINANCE", lang="zh"),
    )
    dump("search", search)

    columns = ["name", "description", "close", "change", "change_abs", "volume", "market_cap_basic"]
    scan = client.scan_symbols(
        "america",
        ["NASDAQ:AAPL", "NASDAQ:MSFT", "NASDAQ:TSLA"],
        columns,
        ScanSymbolsOptions(lang="zh"),
    )
    dump("stocks", client.map_scan_rows(scan, columns))

    forex = client.get_forex_quotes(["OANDA:EURUSD", "OANDA:GBPUSD"], QUOTE_FIELDS)
    dump("forex", client.map_scan_rows(forex, QUOTE_FIELDS))


if __name__ == "__main__":
    try:
        main()
    except TradingViewError as exc:
        print(f"请求失败：{exc}", file=sys.stderr)
        print('如果直连 TradingView 超时，请配置代理，例如：$env:TRADINGVIEW_PROXY="http://127.0.0.1:7890"; python .\\examples\\basic.py', file=sys.stderr)
        raise SystemExit(1)
