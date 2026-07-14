package bridge

// Typed wrappers for methods used by `mt5-pp-cli sync`. Loose maps in many places
// because MT5 keeps adding fields broker-by-broker — strict structs would
// reject unknown keys; the SQL schema only stores what it cares about.

// SymbolInfo mirrors mt5.symbol_info()._asdict() — only the fields the store
// schema persists; MT5 returns many more.
type SymbolInfo struct {
	Name              string  `json:"name"`
	Description       string  `json:"description"`
	Digits            int     `json:"digits"`
	Point             float64 `json:"point"`
	Spread            int     `json:"spread"`
	TradeContractSize float64 `json:"trade_contract_size"`
	VolumeMin         float64 `json:"volume_min"`
	VolumeMax         float64 `json:"volume_max"`
	VolumeStep        float64 `json:"volume_step"`
	TradeMode         int     `json:"trade_mode"`
	TradeCalcMode     int     `json:"trade_calc_mode"`
	CurrencyBase      string  `json:"currency_base"`
	CurrencyProfit    string  `json:"currency_profit"`
	MarginInitial     float64 `json:"margin_initial"`
}

// Position mirrors mt5.positions_get() row.
type Position struct {
	Ticket       int64   `json:"ticket"`
	Time         int64   `json:"time"` // unix seconds
	TimeUpdate   int64   `json:"time_update"`
	Type         int     `json:"type"` // 0=buy, 1=sell
	Magic        int64   `json:"magic"`
	Identifier   int64   `json:"identifier"`
	Reason       int     `json:"reason"`
	Volume       float64 `json:"volume"`
	PriceOpen    float64 `json:"price_open"`
	SL           float64 `json:"sl"`
	TP           float64 `json:"tp"`
	PriceCurrent float64 `json:"price_current"`
	Swap         float64 `json:"swap"`
	Profit       float64 `json:"profit"`
	Symbol       string  `json:"symbol"`
	Comment      string  `json:"comment"`
}

// Order mirrors mt5.orders_get() row (active orders).
type Order struct {
	Ticket         int64   `json:"ticket"`
	TimeSetup      int64   `json:"time_setup"`
	TimeSetupMSC   int64   `json:"time_setup_msc"`
	TimeExpiration int64   `json:"time_expiration"`
	Type           int     `json:"type"`
	TypeTime       int     `json:"type_time"`
	TypeFilling    int     `json:"type_filling"`
	State          int     `json:"state"`
	Magic          int64   `json:"magic"`
	PositionID     int64   `json:"position_id"`
	VolumeInitial  float64 `json:"volume_initial"`
	VolumeCurrent  float64 `json:"volume_current"`
	PriceOpen      float64 `json:"price_open"`
	SL             float64 `json:"sl"`
	TP             float64 `json:"tp"`
	PriceCurrent   float64 `json:"price_current"`
	Symbol         string  `json:"symbol"`
	Comment        string  `json:"comment"`
}

// Deal mirrors mt5.history_deals_get() row.
type Deal struct {
	Ticket     int64   `json:"ticket"`
	Order      int64   `json:"order"`
	Time       int64   `json:"time"`
	TimeMSC    int64   `json:"time_msc"`
	Type       int     `json:"type"`
	Entry      int     `json:"entry"` // 0=in 1=out 2=inout 3=out_by
	Magic      int64   `json:"magic"`
	PositionID int64   `json:"position_id"`
	Volume     float64 `json:"volume"`
	Price      float64 `json:"price"`
	Commission float64 `json:"commission"`
	Swap       float64 `json:"swap"`
	Profit     float64 `json:"profit"`
	Fee        float64 `json:"fee"`
	Symbol     string  `json:"symbol"`
	Comment    string  `json:"comment"`
}

// HistoryOrder mirrors mt5.history_orders_get() row.
type HistoryOrder struct {
	Ticket        int64   `json:"ticket"`
	TimeSetup     int64   `json:"time_setup"`
	TimeDone      int64   `json:"time_done"`
	Type          int     `json:"type"`
	State         int     `json:"state"`
	Magic         int64   `json:"magic"`
	PositionID    int64   `json:"position_id"`
	VolumeInitial float64 `json:"volume_initial"`
	VolumeCurrent float64 `json:"volume_current"`
	PriceOpen     float64 `json:"price_open"`
	SL            float64 `json:"sl"`
	TP            float64 `json:"tp"`
	Symbol        string  `json:"symbol"`
	Comment       string  `json:"comment"`
}

// Bar mirrors one row of mt5.copy_rates_range(). 'time' is unix seconds.
type Bar struct {
	Time       int64   `json:"time"`
	Open       float64 `json:"open"`
	High       float64 `json:"high"`
	Low        float64 `json:"low"`
	Close      float64 `json:"close"`
	TickVolume int64   `json:"tick_volume"`
	Spread     int     `json:"spread"`
	RealVolume int64   `json:"real_volume"`
}

// Tick mirrors one row of mt5.copy_ticks_range(). 'time' is unix seconds;
// 'time_msc' is milliseconds.
type Tick struct {
	Time       int64   `json:"time"`
	TimeMSC    int64   `json:"time_msc"`
	Bid        float64 `json:"bid"`
	Ask        float64 `json:"ask"`
	Last       float64 `json:"last"`
	Volume     float64 `json:"volume"`
	Flags      int     `json:"flags"`
	VolumeReal float64 `json:"volume_real"`
}

// ── high-level wrappers ──────────────────────────────────────────────────────

// SymbolsGet returns the visible symbol catalog. Optional `group` filter
// (e.g. "EUR*") is passed straight through.
func (b *Bridge) SymbolsGet(group string) ([]SymbolInfo, error) {
	params := map[string]any{}
	if group != "" {
		params["group"] = group
	}
	var out []SymbolInfo
	if err := b.Call("symbols_get", params, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PositionsGet returns currently open positions, optionally filtered.
func (b *Bridge) PositionsGet(filter map[string]any) ([]Position, error) {
	if filter == nil {
		filter = map[string]any{}
	}
	var out []Position
	if err := b.Call("positions_get", filter, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// OrdersGet returns active (pending) orders.
func (b *Bridge) OrdersGet(filter map[string]any) ([]Order, error) {
	if filter == nil {
		filter = map[string]any{}
	}
	var out []Order
	if err := b.Call("orders_get", filter, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// HistoryDealsGet returns historical deals between from/to (unix seconds).
func (b *Bridge) HistoryDealsGet(fromUnix, toUnix int64) ([]Deal, error) {
	var out []Deal
	if err := b.Call("history_deals_get", map[string]any{
		"date_from": fromUnix, "date_to": toUnix,
	}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// HistoryOrdersGet returns historical orders between from/to (unix seconds).
func (b *Bridge) HistoryOrdersGet(fromUnix, toUnix int64) ([]HistoryOrder, error) {
	var out []HistoryOrder
	if err := b.Call("history_orders_get", map[string]any{
		"date_from": fromUnix, "date_to": toUnix,
	}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CopyRatesRange returns bars for [from,to] at the given timeframe ("M5", "H1"...).
func (b *Bridge) CopyRatesRange(symbol, timeframe string, fromUnix, toUnix int64) ([]Bar, error) {
	var out []Bar
	if err := b.Call("copy_rates_range", map[string]any{
		"symbol": symbol, "timeframe": timeframe,
		"date_from": fromUnix, "date_to": toUnix,
	}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CopyTicksRange returns ticks for [from,to]. flags = "all" | "info" | "trade".
func (b *Bridge) CopyTicksRange(symbol string, fromUnix, toUnix int64, flags string) ([]Tick, error) {
	if flags == "" {
		flags = "all"
	}
	var out []Tick
	if err := b.Call("copy_ticks_range", map[string]any{
		"symbol":    symbol,
		"date_from": fromUnix, "date_to": toUnix,
		"flags": flags,
	}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ── live single-symbol reads ─────────────────────────────────────────────────

// SymbolInfoFull mirrors mt5.symbol_info()._asdict() with the fields we read
// out. Many more exist; broker-specific ones are deliberately ignored.
type SymbolInfoFull struct {
	Name              string  `json:"name"`
	Description       string  `json:"description"`
	Currency          string  `json:"currency_base"`
	CurrencyProfit    string  `json:"currency_profit"`
	Digits            int     `json:"digits"`
	Point             float64 `json:"point"`
	Spread            int     `json:"spread"`
	SpreadFloat       bool    `json:"spread_float"`
	TradeContractSize float64 `json:"trade_contract_size"`
	TradeTickSize     float64 `json:"trade_tick_size"`
	TradeTickValue    float64 `json:"trade_tick_value"`
	VolumeMin         float64 `json:"volume_min"`
	VolumeMax         float64 `json:"volume_max"`
	VolumeStep        float64 `json:"volume_step"`
	TradeMode         int     `json:"trade_mode"`
	Bid               float64 `json:"bid"`
	Ask               float64 `json:"ask"`
	Last              float64 `json:"last"`
	Time              int64   `json:"time"`
	Visible           bool    `json:"visible"`
	Select            bool    `json:"select"`
}

// SymbolTick is the snapshot returned by mt5.symbol_info_tick().
type SymbolTick struct {
	Time       int64   `json:"time"`
	TimeMSC    int64   `json:"time_msc"`
	Bid        float64 `json:"bid"`
	Ask        float64 `json:"ask"`
	Last       float64 `json:"last"`
	Volume     float64 `json:"volume"`
	Flags      int     `json:"flags"`
	VolumeReal float64 `json:"volume_real"`
}

// BookItem is one rung of the depth-of-market book.
type BookItem struct {
	Type   int     `json:"type"` // 1=sell 2=buy 3=sell_market 4=buy_market 5=sell_limit 6=buy_limit 7=sell_stop 8=buy_stop
	Price  float64 `json:"price"`
	Volume float64 `json:"volume"`
}

// SymbolInfo fetches the full symbol record (live).
func (b *Bridge) SymbolInfo(symbol string) (*SymbolInfoFull, error) {
	var out SymbolInfoFull
	if err := b.Call("symbol_info", map[string]any{"symbol": symbol}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SymbolInfoTick returns the most recent tick for the symbol.
func (b *Bridge) SymbolInfoTick(symbol string) (*SymbolTick, error) {
	var out SymbolTick
	if err := b.Call("symbol_info_tick", map[string]any{"symbol": symbol}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MarketBookGet returns the current depth book for symbol. Caller is
// responsible for subscribing/unsubscribing via the matching add/release; for
// a one-shot read this call subscribes implicitly.
func (b *Bridge) MarketBookGet(symbol string) ([]BookItem, error) {
	var out []BookItem
	if err := b.Call("market_book_get", map[string]any{"symbol": symbol}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// OrderCalcMargin calls mt5.order_calc_margin(action, symbol, volume, price).
// action: 0=BUY 1=SELL (and the limit/stop variants if used).
func (b *Bridge) OrderCalcMargin(action int, symbol string, volume, price float64) (float64, error) {
	var out float64
	if err := b.Call("order_calc_margin", map[string]any{
		"action": action, "symbol": symbol, "volume": volume, "price": price,
	}, &out); err != nil {
		return 0, err
	}
	return out, nil
}

// OrderCalcProfit calls mt5.order_calc_profit(action, symbol, volume, price_open, price_close).
func (b *Bridge) OrderCalcProfit(action int, symbol string, volume, priceOpen, priceClose float64) (float64, error) {
	var out float64
	if err := b.Call("order_calc_profit", map[string]any{
		"action": action, "symbol": symbol, "volume": volume,
		"price_open": priceOpen, "price_close": priceClose,
	}, &out); err != nil {
		return 0, err
	}
	return out, nil
}

// OrderSendResult is mt5.order_send()'s result. retcode 10009/10008 = done/placed.
type OrderSendResult struct {
	Retcode    int     `json:"retcode"`
	Deal       int64   `json:"deal"`
	Order      int64   `json:"order"`
	Volume     float64 `json:"volume"`
	Price      float64 `json:"price"`
	Bid        float64 `json:"bid"`
	Ask        float64 `json:"ask"`
	Comment    string  `json:"comment"`
	RequestID  int64   `json:"request_id"`
	RetcodeExt int     `json:"retcode_external"`
}

// OrderSend executes mt5.order_send with the given pre-built request map.
// Caller owns building the request shape — this is the unconstrained escape
// hatch the write commands use. Returns (nil, ErrBrokerRejected wrapper) when
// retcode is not 10008/10009.
func (b *Bridge) OrderSend(req map[string]any) (*OrderSendResult, error) {
	var out OrderSendResult
	if err := b.Call("order_send", map[string]any{"request": req}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// OrderCheck previews via mt5.order_check — never sends.
func (b *Bridge) OrderCheck(req map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := b.Call("order_check", map[string]any{"request": req}, &out); err != nil {
		return nil, err
	}
	return out, nil
}
