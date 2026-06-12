from __future__ import annotations

import gzip
import json
import ssl
from dataclasses import dataclass, field
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode, urlparse
from urllib.request import (
    HTTPSHandler,
    ProxyHandler,
    Request,
    build_opener,
    urlopen,
)

from .field_groups import (
    DETAIL_FIELDS,
    PERFORMANCE_FIELDS,
    QUOTE_FIELDS,
    RELATED_SYMBOL_COLUMNS,
    TECHNICAL_FIELDS,
)

DEFAULT_BASE_URL = "https://scanner.tradingview.com"
DEFAULT_SYMBOL_SEARCH_BASE_URL = "https://symbol-search.tradingview.com"
DEFAULT_USER_AGENT = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
DEFAULT_ORIGIN = "https://cn.tradingview.com"
DEFAULT_REFERER = "https://cn.tradingview.com/"


class TradingViewError(RuntimeError):
    pass


@dataclass
class ProxyConfig:
    mode: str = "balance"
    primary: str = ""
    backup: list[str] = field(default_factory=list)
    proxies: list[str] = field(default_factory=list)


@dataclass
class ClientOptions:
    base_url: str = DEFAULT_BASE_URL
    symbol_search_base_url: str = DEFAULT_SYMBOL_SEARCH_BASE_URL
    user_agent: str = DEFAULT_USER_AGENT
    origin: str = DEFAULT_ORIGIN
    referer: str = DEFAULT_REFERER
    timeout: int = 20
    connect_timeout: int = 10
    verify_ssl: bool = True
    proxy: str | ProxyConfig | dict[str, Any] | None = None
    headers: dict[str, str] = field(default_factory=dict)


@dataclass
class SymbolFieldsOptions:
    no_404: bool = True
    label_product: str | None = None
    headers: dict[str, str] = field(default_factory=dict)


@dataclass
class SearchSymbolsOptions:
    exchange: str = ""
    search_type: str | None = "undefined"
    type: str | None = None
    lang: str = "zh"
    hl: bool | int | str = True
    domain: str = "production"
    enable_grouping: bool | str = True
    sort_by_country: str | None = "CN"
    promo: bool | str = True
    extra_query: dict[str, Any] = field(default_factory=dict)
    headers: dict[str, str] = field(default_factory=dict)


@dataclass
class ScanSymbolsOptions:
    range: list[int] | None = None
    types: list[Any] = field(default_factory=list)
    label_product: str | None = None
    sort: dict[str, Any] | None = None
    ignore_unknown_fields: bool | None = None
    lang: str | None = None
    headers: dict[str, str] = field(default_factory=dict)


@dataclass
class ForexRelatedOptions:
    lang: str = "zh"
    label_product: str | None = "related-symbols"
    headers: dict[str, str] = field(default_factory=dict)


class TradingViewClient:
    def __init__(self, options: ClientOptions | dict[str, Any] | None = None):
        if options is None:
            options = ClientOptions()
        elif isinstance(options, dict):
            options = ClientOptions(**options)

        self.base_url = options.base_url.rstrip("/")
        self.symbol_search_base_url = options.symbol_search_base_url.rstrip("/")
        self.user_agent = options.user_agent
        self.origin = options.origin
        self.referer = options.referer
        self.timeout = options.timeout
        self.connect_timeout = options.connect_timeout
        self.verify_ssl = options.verify_ssl
        self.default_headers = _normalize_headers(
            {
                "accept": "application/json",
                "origin": self.origin,
                "referer": self.referer,
                "user-agent": self.user_agent,
                **options.headers,
            }
        )
        self.proxy_mode = "none"
        self.proxies: list[str] = []
        self.balance_proxy_cursor = 0
        self._configure_proxy(options.proxy)

    def search_symbols(
        self,
        text: str,
        options: SearchSymbolsOptions | dict[str, Any] | None = None,
    ) -> list[Any]:
        text = text.strip()
        if not text:
            raise TradingViewError("text 不能为空。")
        options = _coerce_options(options, SearchSymbolsOptions)

        query: dict[str, Any] = {
            "text": text,
            "hl": _format_query_flag(options.hl, "1", "0"),
            "exchange": options.exchange,
            "lang": options.lang,
            "domain": options.domain,
            "enable_grouping": _format_query_flag(options.enable_grouping),
            "promo": _format_query_flag(options.promo),
        }

        search_type = options.search_type if options.search_type not in (None, "") else options.type
        if search_type not in (None, ""):
            query["search_type"] = search_type

        if options.sort_by_country not in (None, ""):
            query["sort_by_country"] = options.sort_by_country

        query.update(options.extra_query)
        headers = {"accept": "*/*", **options.headers}
        result = self._request(
            "GET",
            "/symbol_search/v3/",
            query=query,
            headers=headers,
            base_url=self.symbol_search_base_url,
        )
        if not isinstance(result, list):
            raise TradingViewError("TradingView 标的搜索返回内容不是数组。")
        return result

    def get_symbol_fields(
        self,
        symbol: str,
        fields: list[str],
        options: SymbolFieldsOptions | dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        _assert_string_list(fields, "fields")
        options = _coerce_options(options, SymbolFieldsOptions)
        query = {
            "symbol": symbol,
            "fields": ",".join(fields),
            "no_404": "true" if options.no_404 else "false",
        }
        if options.label_product:
            query["label-product"] = options.label_product
        result = self._request("GET", "/symbol", query=query, headers=options.headers)
        if not isinstance(result, dict):
            raise TradingViewError("TradingView /symbol 返回内容不是对象。")
        return result

    def get_symbol_quote(
        self,
        symbol: str,
        fields: list[str] | None = None,
        options: SymbolFieldsOptions | dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        options = _coerce_options(options, SymbolFieldsOptions)
        if not options.label_product:
            options.label_product = "right-details"
        return self.get_symbol_fields(symbol, fields or QUOTE_FIELDS, options)

    def get_symbol_details(
        self,
        symbol: str,
        fields: list[str] | None = None,
        options: SymbolFieldsOptions | dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        options = _coerce_options(options, SymbolFieldsOptions)
        if not options.label_product:
            options.label_product = "right-details"
        return self.get_symbol_fields(symbol, fields or DETAIL_FIELDS, options)

    def get_symbol_performance(
        self,
        symbol: str,
        fields: list[str] | None = None,
        options: SymbolFieldsOptions | dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        options = _coerce_options(options, SymbolFieldsOptions)
        if not options.label_product:
            options.label_product = "symbols-performance"
        return self.get_symbol_fields(symbol, fields or PERFORMANCE_FIELDS, options)

    def get_symbol_technicals(
        self,
        symbol: str,
        fields: list[str] | None = None,
        options: SymbolFieldsOptions | dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        options = _coerce_options(options, SymbolFieldsOptions)
        if not options.label_product:
            options.label_product = "symbols-technicals"
        return self.get_symbol_fields(symbol, fields or TECHNICAL_FIELDS, options)

    def scan(
        self,
        market: str,
        payload: dict[str, Any],
        label_product: str | None = None,
        headers: dict[str, str] | None = None,
    ) -> dict[str, Any]:
        market = market.strip("/ \t\n\r\x00\v")
        if not market:
            raise TradingViewError("market 不能为空。")
        query = {}
        if label_product:
            query["label-product"] = label_product
        request_headers = {
            "content-type": "application/json",
            "accept": "application/json",
            **(headers or {}),
        }
        result = self._request(
            "POST",
            f"/{market}/scan",
            query=query,
            body=payload,
            headers=request_headers,
        )
        if not isinstance(result, dict):
            raise TradingViewError("TradingView scan 返回内容不是对象。")
        return result

    def scan_symbols(
        self,
        market: str,
        tickers: list[str],
        columns: list[str],
        options: ScanSymbolsOptions | dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        _assert_string_list(tickers, "tickers")
        _assert_string_list(columns, "columns")
        options = _coerce_options(options, ScanSymbolsOptions)
        payload: dict[str, Any] = {
            "symbols": {
                "tickers": list(tickers),
                "query": {
                    "types": options.types,
                },
            },
            "columns": list(columns),
            "range": options.range if options.range is not None else [0, len(tickers)],
        }
        if options.sort is not None:
            payload["sort"] = options.sort
        if options.ignore_unknown_fields is not None:
            payload["ignore_unknown_fields"] = bool(options.ignore_unknown_fields)
        if options.lang is not None:
            payload["options"] = {"lang": options.lang}
        return self.scan(market, payload, options.label_product, options.headers)

    def get_forex_quotes(
        self,
        tickers: list[str],
        columns: list[str] | None = None,
        options: ScanSymbolsOptions | dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        return self.scan_symbols("forex", tickers, columns or QUOTE_FIELDS, options)

    def scan_forex_related_symbols(
        self,
        currency_id: str,
        exclude_name: str,
        counter_currencies: list[str] | None = None,
        exchange: str = "FX_IDC",
        columns: list[str] | None = None,
        limit: int = 11,
        options: ForexRelatedOptions | dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        counter_currencies = counter_currencies or ["USD", "EUR", "JPY", "GBP", "CHF", "CNY"]
        columns = columns or RELATED_SYMBOL_COLUMNS
        options = _coerce_options(options, ForexRelatedOptions)
        _assert_string_list(counter_currencies, "counter_currencies")
        _assert_string_list(columns, "columns")

        payload = {
            "columns": list(columns),
            "ignore_unknown_fields": True,
            "options": {
                "lang": options.lang,
            },
            "range": [0, limit],
            "sort": {
                "sortBy": "popularity_rank",
                "sortOrder": "asc",
            },
            "filter2": {
                "operator": "and",
                "operands": [
                    {"expression": {"left": "type", "operation": "equal", "right": "forex"}},
                    {"expression": {"left": "exchange", "operation": "equal", "right": exchange}},
                    {"expression": {"left": "name", "operation": "nequal", "right": exclude_name}},
                    {
                        "operation": {
                            "operator": "or",
                            "operands": [
                                {
                                    "operation": {
                                        "operator": "and",
                                        "operands": [
                                            {
                                                "expression": {
                                                    "left": "currency_id",
                                                    "operation": "equal",
                                                    "right": currency_id,
                                                }
                                            },
                                            {
                                                "expression": {
                                                    "left": "base_currency_id",
                                                    "operation": "in_range",
                                                    "right": list(counter_currencies),
                                                }
                                            },
                                        ],
                                    }
                                },
                                {
                                    "operation": {
                                        "operator": "and",
                                        "operands": [
                                            {
                                                "expression": {
                                                    "left": "base_currency_id",
                                                    "operation": "equal",
                                                    "right": currency_id,
                                                }
                                            },
                                            {
                                                "expression": {
                                                    "left": "currency_id",
                                                    "operation": "in_range",
                                                    "right": list(counter_currencies),
                                                }
                                            },
                                        ],
                                    }
                                },
                            ],
                        }
                    },
                ],
            },
        }

        return self.scan("forex", payload, options.label_product, options.headers)

    def map_scan_rows(self, response: dict[str, Any], columns: list[str]) -> list[dict[str, Any]]:
        return map_scan_rows(response, columns)

    def convert_recommend_value(self, value: float | int | None) -> str:
        return convert_recommend_value(value)

    def _request(
        self,
        method: str,
        path: str,
        query: dict[str, Any] | None = None,
        body: dict[str, Any] | None = None,
        headers: dict[str, str] | None = None,
        base_url: str | None = None,
    ) -> Any:
        url = _build_url(base_url or self.base_url, path, query or {})
        data = None
        if body is not None:
            data = json.dumps(body, ensure_ascii=False, separators=(",", ":")).encode("utf-8")

        attempts = self._build_proxy_attempts()
        last_error: Exception | None = None
        for proxy in attempts:
            try:
                return self._send(method, url, data, headers or {}, proxy)
            except TradingViewError as error:
                last_error = error
        if last_error:
            raise last_error
        raise TradingViewError("没有可用代理。")

    def _send(
        self,
        method: str,
        url: str,
        data: bytes | None,
        headers: dict[str, str],
        proxy: str | None,
    ) -> Any:
        merged_headers = _normalize_headers({**self.default_headers, **headers})
        merged_headers.setdefault("accept-encoding", "gzip")
        request = Request(url, data=data, method=method.upper(), headers=merged_headers)
        context = ssl.create_default_context()
        if not self.verify_ssl:
            context = ssl._create_unverified_context()

        opener = None
        if proxy:
            opener = build_opener(
                ProxyHandler({"http": proxy, "https": proxy}),
                HTTPSHandler(context=context),
            )

        try:
            if opener:
                response = opener.open(request, timeout=self.timeout)
            else:
                response = urlopen(request, timeout=self.timeout, context=context)
            with response:
                raw = response.read()
                if response.headers.get("Content-Encoding", "").lower() == "gzip":
                    raw = gzip.decompress(raw)
                return json.loads(raw.decode("utf-8"))
        except HTTPError as error:
            body = error.read().decode("utf-8", errors="replace")
            raise TradingViewError(f"TradingView 返回 HTTP {error.code}：{url}；响应：{body}") from error
        except (URLError, TimeoutError, OSError) as error:
            raise TradingViewError(f"请求 TradingView 失败：{error}") from error
        except json.JSONDecodeError as error:
            raise TradingViewError(f"TradingView 返回内容不是有效 JSON：{error}") from error

    def _configure_proxy(self, proxy: str | ProxyConfig | dict[str, Any] | None) -> None:
        if proxy in (None, ""):
            self.proxy_mode = "none"
            self.proxies = []
            return
        if isinstance(proxy, str):
            self.proxy_mode = "single"
            self.proxies = [_normalize_proxy_url(proxy, "proxy")]
            return
        if isinstance(proxy, dict):
            proxy = ProxyConfig(**proxy)
        if not isinstance(proxy, ProxyConfig):
            raise TradingViewError("proxy 配置必须是字符串、ProxyConfig、dict 或 None。")

        mode = _normalize_proxy_mode(proxy.mode or "balance")
        if mode == "balance":
            self.proxy_mode = "balance"
            self.proxies = _normalize_proxy_list(proxy.proxies, "proxy.proxies", allow_empty=False)
            return
        if mode == "failover":
            primary = _normalize_proxy_url(proxy.primary, "proxy.primary")
            backup = _normalize_proxy_list(proxy.backup, "proxy.backup", allow_empty=True)
            self.proxy_mode = "failover"
            self.proxies = [primary, *backup]
            return
        raise TradingViewError("proxy.mode 只支持 balance 或 failover。")

    def _build_proxy_attempts(self) -> list[str | None]:
        if self.proxy_mode == "none":
            return [None]
        if self.proxy_mode in ("single", "failover"):
            return list(self.proxies)
        if self.proxy_mode == "balance":
            if not self.proxies:
                raise TradingViewError("proxy.proxies 不能为空。")
            start = self.balance_proxy_cursor % len(self.proxies)
            self.balance_proxy_cursor = (self.balance_proxy_cursor + 1) % len(self.proxies)
            return [*self.proxies[start:], *self.proxies[:start]]
        raise TradingViewError(f"未知代理模式：{self.proxy_mode}")


def map_scan_rows(response: dict[str, Any], columns: list[str]) -> list[dict[str, Any]]:
    _assert_string_list(columns, "columns")
    rows: list[dict[str, Any]] = []
    for item in response.get("data") or []:
        if not isinstance(item, dict):
            continue
        values = item.get("d")
        if not isinstance(values, list):
            values = []
        mapped = {
            "symbol": item.get("s"),
            "raw": values,
        }
        for index, column in enumerate(columns):
            mapped[column] = values[index] if index < len(values) else None
        rows.append(mapped)
    return rows


def convert_recommend_value(value: float | int | None) -> str:
    if value is None:
        return "N/A"
    if value <= -0.5:
        return "Strong Sell"
    if value <= -0.1:
        return "Sell"
    if value < 0.1:
        return "Neutral"
    if value < 0.5:
        return "Buy"
    return "Strong Buy"


def _build_url(base_url: str, path: str, query: dict[str, Any]) -> str:
    url = base_url.rstrip("/") + "/" + path.lstrip("/")
    clean_query = {key: str(value) for key, value in query.items() if value is not None}
    if not clean_query:
        return url
    return url + "?" + urlencode(clean_query)


def _format_query_flag(value: bool | int | str, true_value: str = "true", false_value: str = "false") -> str:
    if isinstance(value, bool):
        return true_value if value else false_value
    return str(value)


def _assert_string_list(values: list[str], name: str) -> None:
    for value in values:
        if not isinstance(value, str) or value == "":
            raise TradingViewError(f"{name} 必须是非空字符串数组。")


def _coerce_options(value: Any, cls: type[Any]) -> Any:
    if value is None:
        return cls()
    if isinstance(value, dict):
        return cls(**value)
    if isinstance(value, cls):
        return value
    raise TradingViewError(f"{cls.__name__} 配置类型不正确。")


def _normalize_headers(headers: dict[str, str]) -> dict[str, str]:
    return {str(name).lower(): str(value) for name, value in headers.items() if value is not None}


def _normalize_proxy_mode(mode: str) -> str:
    mode = mode.strip().lower()
    if mode in ("balance", "balanced", "round_robin", "round-robin", "均衡"):
        return "balance"
    if mode in ("failover", "primary_backup", "primary-backup", "backup", "主备"):
        return "failover"
    return mode


def _normalize_proxy_list(values: list[str], name: str, allow_empty: bool = False) -> list[str]:
    proxies = [_normalize_proxy_url(value, f"{name}[{index}]") for index, value in enumerate(values)]
    if not allow_empty and not proxies:
        raise TradingViewError(f"{name} 不能为空。")
    return proxies


def _normalize_proxy_url(proxy: str, name: str) -> str:
    if not isinstance(proxy, str):
        raise TradingViewError(f"{name} 必须是非空字符串。")
    proxy = proxy.strip()
    if not proxy:
        raise TradingViewError(f"{name} 必须是非空字符串。")
    if proxy.lower().startswith("s5://"):
        proxy = "socks5://" + proxy[5:]
    scheme = urlparse(proxy).scheme.lower()
    if scheme not in ("http", "https", "socks5", "socks5h"):
        raise TradingViewError(f"{name} 只支持 http、https、socks5 或 s5 代理地址。")
    return proxy
