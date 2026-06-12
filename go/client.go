package tradingview

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL             = "https://scanner.tradingview.com"
	defaultSymbolSearchBaseURL = "https://symbol-search.tradingview.com"
	defaultUserAgent           = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
	defaultOrigin              = "https://cn.tradingview.com"
	defaultReferer             = "https://cn.tradingview.com/"
)

type TradingViewError struct {
	Message string
}

func (e *TradingViewError) Error() string {
	return e.Message
}

type ProxyConfig struct {
	Mode    string
	Primary string
	Backup  []string
	Proxies []string
}

type ClientOptions struct {
	BaseURL             string
	SymbolSearchBaseURL string
	UserAgent           string
	Origin              string
	Referer             string
	Timeout             time.Duration
	ConnectTimeout      time.Duration
	VerifySSL           *bool
	Headers             map[string]string
	Proxy               string
	ProxyPool           *ProxyConfig
	HTTPClient          *http.Client
}

type Client struct {
	baseURL             string
	symbolSearchBaseURL string
	userAgent           string
	origin              string
	referer             string
	timeout             time.Duration
	connectTimeout      time.Duration
	verifySSL           bool
	defaultHeaders      map[string]string
	httpClient          *http.Client
	proxyMode           string
	proxies             []string
	balanceProxyCursor  int
}

type SymbolFieldsOptions struct {
	No404        *bool
	LabelProduct string
	Headers      map[string]string
}

type SearchSymbolsOptions struct {
	Exchange       string
	SearchType     string
	Type           string
	Lang           string
	HL             any
	Domain         string
	EnableGrouping any
	SortByCountry  *string
	Promo          any
	ExtraQuery     map[string]any
	Headers        map[string]string
}

type ScanOptions struct {
	Headers map[string]string
}

type ScanSymbolsOptions struct {
	Range               []int
	Types               []any
	LabelProduct        string
	Sort                map[string]any
	IgnoreUnknownFields *bool
	Lang                string
	Headers             map[string]string
}

type ForexRelatedOptions struct {
	Lang         string
	LabelProduct string
	Headers      map[string]string
}

func NewClient(options *ClientOptions) (*Client, error) {
	if options == nil {
		options = &ClientOptions{}
	}

	timeout := options.Timeout
	if timeout == 0 {
		timeout = 20 * time.Second
	}

	connectTimeout := options.ConnectTimeout
	if connectTimeout == 0 {
		connectTimeout = 10 * time.Second
	}

	verifySSL := true
	if options.VerifySSL != nil {
		verifySSL = *options.VerifySSL
	}

	client := &Client{
		baseURL:             trimRightSlash(firstNonEmpty(options.BaseURL, defaultBaseURL)),
		symbolSearchBaseURL: trimRightSlash(firstNonEmpty(options.SymbolSearchBaseURL, defaultSymbolSearchBaseURL)),
		userAgent:           firstNonEmpty(options.UserAgent, defaultUserAgent),
		origin:              firstNonEmpty(options.Origin, defaultOrigin),
		referer:             firstNonEmpty(options.Referer, defaultReferer),
		timeout:             timeout,
		connectTimeout:      connectTimeout,
		verifySSL:           verifySSL,
		defaultHeaders: map[string]string{
			"accept":     "application/json",
			"origin":     firstNonEmpty(options.Origin, defaultOrigin),
			"referer":    firstNonEmpty(options.Referer, defaultReferer),
			"user-agent": firstNonEmpty(options.UserAgent, defaultUserAgent),
		},
	}

	for name, value := range options.Headers {
		if value != "" {
			client.defaultHeaders[strings.ToLower(name)] = value
		}
	}

	if err := client.configureProxy(options.Proxy, options.ProxyPool); err != nil {
		return nil, err
	}

	if options.HTTPClient != nil {
		client.httpClient = options.HTTPClient
	} else {
		client.httpClient = client.newHTTPClient(nil)
	}

	return client, nil
}

func (c *Client) SearchSymbols(ctx context.Context, text string, options *SearchSymbolsOptions) ([]any, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, &TradingViewError{Message: "text 不能为空。"}
	}

	if options == nil {
		options = &SearchSymbolsOptions{}
	}

	query := map[string]any{
		"text":            text,
		"hl":              formatQueryFlag(defaultAny(options.HL, true), "1", "0"),
		"exchange":        options.Exchange,
		"lang":            firstNonEmpty(options.Lang, "zh"),
		"domain":          firstNonEmpty(options.Domain, "production"),
		"enable_grouping": formatQueryFlag(defaultAny(options.EnableGrouping, true), "true", "false"),
		"promo":           formatQueryFlag(defaultAny(options.Promo, true), "true", "false"),
	}

	searchType := options.SearchType
	if searchType == "" {
		searchType = options.Type
	}
	if searchType == "" {
		searchType = "undefined"
	}
	if searchType != "" {
		query["search_type"] = searchType
	}

	if options.SortByCountry == nil {
		query["sort_by_country"] = "CN"
	} else if *options.SortByCountry != "" {
		query["sort_by_country"] = *options.SortByCountry
	}

	for key, value := range options.ExtraQuery {
		query[key] = value
	}

	headers := mergeHeaders(map[string]string{"accept": "*/*"}, options.Headers)
	var decoded any
	if err := c.request(ctx, http.MethodGet, "/symbol_search/v3/", query, nil, headers, c.symbolSearchBaseURL, &decoded); err != nil {
		return nil, err
	}

	items, ok := decoded.([]any)
	if !ok {
		return nil, &TradingViewError{Message: "TradingView 标的搜索返回内容不是数组。"}
	}

	return items, nil
}

func (c *Client) GetSymbolFields(ctx context.Context, symbol string, fields []string, options *SymbolFieldsOptions) (map[string]any, error) {
	if err := assertStringList(fields, "fields"); err != nil {
		return nil, err
	}

	if options == nil {
		options = &SymbolFieldsOptions{}
	}

	no404 := true
	if options.No404 != nil {
		no404 = *options.No404
	}

	query := map[string]any{
		"symbol": symbol,
		"fields": strings.Join(fields, ","),
		"no_404": strconv.FormatBool(no404),
	}

	if options.LabelProduct != "" {
		query["label-product"] = options.LabelProduct
	}

	var decoded map[string]any
	err := c.request(ctx, http.MethodGet, "/symbol", query, nil, options.Headers, "", &decoded)
	return decoded, err
}

func (c *Client) GetSymbolQuote(ctx context.Context, symbol string, fields []string, options *SymbolFieldsOptions) (map[string]any, error) {
	if len(fields) == 0 {
		fields = QuoteFields
	}
	if options == nil {
		options = &SymbolFieldsOptions{}
	}
	if options.LabelProduct == "" {
		options.LabelProduct = "right-details"
	}
	return c.GetSymbolFields(ctx, symbol, fields, options)
}

func (c *Client) GetSymbolDetails(ctx context.Context, symbol string, fields []string, options *SymbolFieldsOptions) (map[string]any, error) {
	if len(fields) == 0 {
		fields = DetailFields
	}
	if options == nil {
		options = &SymbolFieldsOptions{}
	}
	if options.LabelProduct == "" {
		options.LabelProduct = "right-details"
	}
	return c.GetSymbolFields(ctx, symbol, fields, options)
}

func (c *Client) GetSymbolPerformance(ctx context.Context, symbol string, fields []string, options *SymbolFieldsOptions) (map[string]any, error) {
	if len(fields) == 0 {
		fields = PerformanceFields
	}
	if options == nil {
		options = &SymbolFieldsOptions{}
	}
	if options.LabelProduct == "" {
		options.LabelProduct = "symbols-performance"
	}
	return c.GetSymbolFields(ctx, symbol, fields, options)
}

func (c *Client) GetSymbolTechnicals(ctx context.Context, symbol string, fields []string, options *SymbolFieldsOptions) (map[string]any, error) {
	if len(fields) == 0 {
		fields = TechnicalFields
	}
	if options == nil {
		options = &SymbolFieldsOptions{}
	}
	if options.LabelProduct == "" {
		options.LabelProduct = "symbols-technicals"
	}
	return c.GetSymbolFields(ctx, symbol, fields, options)
}

func (c *Client) Scan(ctx context.Context, market string, payload map[string]any, labelProduct string, options *ScanOptions) (map[string]any, error) {
	market = strings.Trim(market, "/ \t\n\r\x00\v")
	if market == "" {
		return nil, &TradingViewError{Message: "market 不能为空。"}
	}
	if options == nil {
		options = &ScanOptions{}
	}

	query := map[string]any{}
	if labelProduct != "" {
		query["label-product"] = labelProduct
	}

	headers := mergeHeaders(map[string]string{
		"content-type": "application/json",
		"accept":       "application/json",
	}, options.Headers)

	var decoded map[string]any
	err := c.request(ctx, http.MethodPost, "/"+market+"/scan", query, payload, headers, "", &decoded)
	return decoded, err
}

func (c *Client) ScanSymbols(ctx context.Context, market string, tickers []string, columns []string, options *ScanSymbolsOptions) (map[string]any, error) {
	if err := assertStringList(tickers, "tickers"); err != nil {
		return nil, err
	}
	if err := assertStringList(columns, "columns"); err != nil {
		return nil, err
	}
	if options == nil {
		options = &ScanSymbolsOptions{}
	}

	queryTypes := []any{}
	if options.Types != nil {
		queryTypes = options.Types
	}

	payload := map[string]any{
		"symbols": map[string]any{
			"tickers": tickers,
			"query": map[string]any{
				"types": queryTypes,
			},
		},
		"columns": columns,
		"range":   []int{0, len(tickers)},
	}

	if len(options.Range) > 0 {
		payload["range"] = options.Range
	}
	if options.Sort != nil {
		payload["sort"] = options.Sort
	}
	if options.IgnoreUnknownFields != nil {
		payload["ignore_unknown_fields"] = *options.IgnoreUnknownFields
	}
	if options.Lang != "" {
		payload["options"] = map[string]any{"lang": options.Lang}
	}

	return c.Scan(ctx, market, payload, options.LabelProduct, &ScanOptions{Headers: options.Headers})
}

func (c *Client) GetForexQuotes(ctx context.Context, tickers []string, columns []string, options *ScanSymbolsOptions) (map[string]any, error) {
	if len(columns) == 0 {
		columns = QuoteFields
	}
	return c.ScanSymbols(ctx, "forex", tickers, columns, options)
}

func (c *Client) ScanForexRelatedSymbols(ctx context.Context, currencyID string, excludeName string, counterCurrencies []string, exchange string, columns []string, limit int, options *ForexRelatedOptions) (map[string]any, error) {
	if len(counterCurrencies) == 0 {
		counterCurrencies = []string{"USD", "EUR", "JPY", "GBP", "CHF", "CNY"}
	}
	if len(columns) == 0 {
		columns = RelatedSymbolColumns
	}
	if exchange == "" {
		exchange = "FX_IDC"
	}
	if limit == 0 {
		limit = 11
	}
	if options == nil {
		options = &ForexRelatedOptions{}
	}
	if err := assertStringList(counterCurrencies, "counterCurrencies"); err != nil {
		return nil, err
	}
	if err := assertStringList(columns, "columns"); err != nil {
		return nil, err
	}

	lang := firstNonEmpty(options.Lang, "zh")
	labelProduct := firstNonEmpty(options.LabelProduct, "related-symbols")
	payload := map[string]any{
		"columns":               columns,
		"ignore_unknown_fields": true,
		"options":               map[string]any{"lang": lang},
		"range":                 []int{0, limit},
		"sort": map[string]any{
			"sortBy":    "popularity_rank",
			"sortOrder": "asc",
		},
		"filter2": map[string]any{
			"operator": "and",
			"operands": []any{
				map[string]any{"expression": map[string]any{"left": "type", "operation": "equal", "right": "forex"}},
				map[string]any{"expression": map[string]any{"left": "exchange", "operation": "equal", "right": exchange}},
				map[string]any{"expression": map[string]any{"left": "name", "operation": "nequal", "right": excludeName}},
				map[string]any{"operation": map[string]any{
					"operator": "or",
					"operands": []any{
						map[string]any{"operation": map[string]any{
							"operator": "and",
							"operands": []any{
								map[string]any{"expression": map[string]any{"left": "currency_id", "operation": "equal", "right": currencyID}},
								map[string]any{"expression": map[string]any{"left": "base_currency_id", "operation": "in_range", "right": counterCurrencies}},
							},
						}},
						map[string]any{"operation": map[string]any{
							"operator": "and",
							"operands": []any{
								map[string]any{"expression": map[string]any{"left": "base_currency_id", "operation": "equal", "right": currencyID}},
								map[string]any{"expression": map[string]any{"left": "currency_id", "operation": "in_range", "right": counterCurrencies}},
							},
						}},
					},
				}},
			},
		},
	}

	return c.Scan(ctx, "forex", payload, labelProduct, &ScanOptions{Headers: options.Headers})
}

func MapScanRows(response map[string]any, columns []string) ([]map[string]any, error) {
	if err := assertStringList(columns, "columns"); err != nil {
		return nil, err
	}

	rows := []map[string]any{}
	data, _ := response["data"].([]any)
	for _, item := range data {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}

		values, _ := itemMap["d"].([]any)
		mapped := map[string]any{
			"symbol": itemMap["s"],
			"raw":    values,
		}

		for index, column := range columns {
			var value any
			if index < len(values) {
				value = values[index]
			}
			mapped[column] = value
		}

		rows = append(rows, mapped)
	}

	return rows, nil
}

func ConvertRecommendValue(value *float64) string {
	if value == nil {
		return "N/A"
	}
	if *value <= -0.5 {
		return "Strong Sell"
	}
	if *value <= -0.1 {
		return "Sell"
	}
	if *value < 0.1 {
		return "Neutral"
	}
	if *value < 0.5 {
		return "Buy"
	}
	return "Strong Buy"
}

func (c *Client) request(ctx context.Context, method string, path string, query map[string]any, body map[string]any, headers map[string]string, baseURL string, target any) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if baseURL == "" {
		baseURL = c.baseURL
	}
	fullURL := buildURL(baseURL, path, query)
	var bodyBytes []byte
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return &TradingViewError{Message: "JSON 编码失败：" + err.Error()}
		}
		bodyBytes = encoded
	}

	attempts, err := c.buildProxyAttempts()
	if err != nil {
		return err
	}
	var lastErr error
	for _, proxy := range attempts {
		client := c.httpClient
		if proxy != "" {
			client = c.newHTTPClient(&proxy)
		}

		var bodyReader io.Reader
		if bodyBytes != nil {
			bodyReader = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
		if err != nil {
			return err
		}
		for name, value := range mergeHeaders(c.defaultHeaders, headers) {
			req.Header.Set(name, value)
		}
		if req.Header.Get("Accept-Encoding") == "" {
			req.Header.Set("Accept-Encoding", "gzip")
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = &TradingViewError{Message: "请求 TradingView 失败：" + err.Error()}
			continue
		}
		err = decodeHTTPResponse(resp, fullURL, target)
		if err != nil {
			lastErr = err
			continue
		}
		return nil
	}

	if lastErr != nil {
		return lastErr
	}

	return &TradingViewError{Message: "没有可用代理。"}
}

func decodeHTTPResponse(resp *http.Response, requestURL string, target any) error {
	defer resp.Body.Close()

	reader := resp.Body
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return err
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &TradingViewError{Message: fmt.Sprintf("TradingView 返回 HTTP %d：%s；响应：%s", resp.StatusCode, requestURL, string(body))}
	}
	if err := json.Unmarshal(body, target); err != nil {
		return &TradingViewError{Message: "TradingView 返回内容不是有效 JSON：" + string(body)}
	}

	return nil
}

func (c *Client) configureProxy(proxy string, proxyPool *ProxyConfig) error {
	if proxy != "" {
		normalized, err := normalizeProxyURL(proxy, "proxy")
		if err != nil {
			return err
		}
		c.proxyMode = "single"
		c.proxies = []string{normalized}
		return nil
	}

	if proxyPool == nil {
		c.proxyMode = "none"
		c.proxies = nil
		return nil
	}

	mode := normalizeProxyMode(firstNonEmpty(proxyPool.Mode, "balance"))
	if mode == "balance" {
		proxies, err := normalizeProxyList(proxyPool.Proxies, "proxy.proxies", false)
		if err != nil {
			return err
		}
		c.proxyMode = "balance"
		c.proxies = proxies
		return nil
	}

	if mode == "failover" {
		primary, err := normalizeProxyURL(proxyPool.Primary, "proxy.primary")
		if err != nil {
			return err
		}
		backup, err := normalizeProxyList(proxyPool.Backup, "proxy.backup", true)
		if err != nil {
			return err
		}
		c.proxyMode = "failover"
		c.proxies = append([]string{primary}, backup...)
		return nil
	}

	return &TradingViewError{Message: "proxy.mode 只支持 balance 或 failover。"}
}

func (c *Client) buildProxyAttempts() ([]string, error) {
	switch c.proxyMode {
	case "", "none":
		return []string{""}, nil
	case "single", "failover":
		return c.proxies, nil
	case "balance":
		count := len(c.proxies)
		if count == 0 {
			return nil, &TradingViewError{Message: "proxy.proxies 不能为空。"}
		}
		start := c.balanceProxyCursor % count
		c.balanceProxyCursor = (c.balanceProxyCursor + 1) % count
		return append(append([]string{}, c.proxies[start:]...), c.proxies[:start]...), nil
	default:
		return nil, &TradingViewError{Message: "未知代理模式：" + c.proxyMode}
	}
}

func (c *Client) newHTTPClient(proxy *string) *http.Client {
	dialer := &net.Dialer{
		Timeout: c.connectTimeout,
	}
	transport := &http.Transport{
		DialContext:     dialer.DialContext,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: !c.verifySSL}, //nolint:gosec
	}
	if proxy != nil && *proxy != "" {
		proxyURL, err := url.Parse(*proxy)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}

	return &http.Client{
		Timeout:   c.timeout,
		Transport: transport,
	}
}

func buildURL(baseURL string, path string, query map[string]any) string {
	fullURL := trimRightSlash(baseURL) + "/" + strings.TrimLeft(path, "/")
	if len(query) == 0 {
		return fullURL
	}

	values := url.Values{}
	for key, value := range query {
		if value == nil {
			continue
		}
		values.Set(key, fmt.Sprint(value))
	}

	return fullURL + "?" + values.Encode()
}

func normalizeProxyMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case "balance", "balanced", "round_robin", "round-robin", "均衡":
		return "balance"
	case "failover", "primary_backup", "primary-backup", "backup", "主备":
		return "failover"
	default:
		return mode
	}
}

func normalizeProxyList(values []string, name string, allowEmpty bool) ([]string, error) {
	proxies := []string{}
	for index, value := range values {
		proxy, err := normalizeProxyURL(value, fmt.Sprintf("%s[%d]", name, index))
		if err != nil {
			return nil, err
		}
		proxies = append(proxies, proxy)
	}

	if !allowEmpty && len(proxies) == 0 {
		return nil, &TradingViewError{Message: name + " 不能为空。"}
	}

	return proxies, nil
}

func normalizeProxyURL(proxy string, name string) (string, error) {
	proxy = strings.TrimSpace(proxy)
	if proxy == "" {
		return "", &TradingViewError{Message: name + " 必须是非空字符串。"}
	}
	if strings.HasPrefix(strings.ToLower(proxy), "s5://") {
		proxy = "socks5://" + proxy[5:]
	}

	parsed, err := url.Parse(proxy)
	if err != nil || parsed.Scheme == "" {
		return "", &TradingViewError{Message: name + " 必须是有效代理地址。"}
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "socks5", "socks5h":
		return proxy, nil
	default:
		return "", &TradingViewError{Message: name + " 只支持 http、https、socks5 或 s5 代理地址。"}
	}
}

func assertStringList(values []string, name string) error {
	for _, value := range values {
		if value == "" {
			return &TradingViewError{Message: name + " 必须是非空字符串数组。"}
		}
	}
	return nil
}

func mergeHeaders(headerSets ...map[string]string) map[string]string {
	merged := map[string]string{}
	for _, headers := range headerSets {
		for name, value := range headers {
			if value == "" {
				continue
			}
			merged[strings.ToLower(name)] = value
		}
	}
	return merged
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func defaultAny(value any, fallback any) any {
	if value == nil {
		return fallback
	}
	return value
}

func formatQueryFlag(value any, trueValue string, falseValue string) string {
	switch v := value.(type) {
	case bool:
		if v {
			return trueValue
		}
		return falseValue
	case nil:
		return falseValue
	default:
		return fmt.Sprint(v)
	}
}

func trimRightSlash(value string) string {
	return strings.TrimRight(value, "/")
}

func IsTradingViewError(err error) bool {
	var target *TradingViewError
	return errors.As(err, &target)
}
