package tradingview

var QuoteFields = []string{
	"name",
	"description",
	"close",
	"bid",
	"ask",
	"change",
	"change_abs",
	"update_mode",
}

var DetailFields = []string{
	"price_52_week_high",
	"price_52_week_low",
	"sector",
	"country",
	"market",
	"Low.1M",
	"High.1M",
	"Perf.W",
	"Perf.1M",
	"Perf.3M",
	"Perf.6M",
	"Perf.Y",
	"Perf.YTD",
	"Recommend.All",
	"average_volume_10d_calc",
	"average_volume_30d_calc",
	"nav_discount_premium",
	"open_interest",
	"country_code_fund",
	"iv",
	"underlying_symbol",
	"delta",
	"gamma",
	"rho",
	"theta",
	"vega",
	"theoPrice",
}

var PerformanceFields = []string{
	"change",
	"Perf.5D",
	"Perf.W",
	"Perf.1M",
	"Perf.6M",
	"Perf.YTD",
	"Perf.Y",
	"Perf.5Y",
	"Perf.10Y",
	"Perf.All",
}

var TechnicalFields = []string{
	"Recommend.Other",
	"Recommend.All",
	"Recommend.MA",
}

var RelatedSymbolColumns = []string{
	"name",
	"type",
	"typespecs",
	"exchange",
	"description",
	"logo",
	"country_code",
	"maturity_date",
	"yield_to_maturity",
	"root",
	"close",
	"current_coupon",
	"coupon_type_general",
}
