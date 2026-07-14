package bridge

// Typed wrappers for the foundation calls. Field names mirror MetaTrader5's
// named-tuple results so a passthrough JSON round-trip is lossless.
//
// Phase 1 covers: Initialize, Login, Shutdown, AccountInfo, TerminalInfo,
// Version, LastError. Additional wrappers land alongside the phase that
// needs them.

// AccountInfo mirrors mt5.account_info()._asdict().
type AccountInfo struct {
	Login             int64   `json:"login"`
	TradeMode         int     `json:"trade_mode"` // 0=demo 1=contest 2=real
	Leverage          int64   `json:"leverage"`
	LimitOrders       int     `json:"limit_orders"`
	MarginSOMode      int     `json:"margin_so_mode"`
	TradeAllowed      bool    `json:"trade_allowed"`
	TradeExpert       bool    `json:"trade_expert"`
	MarginMode        int     `json:"margin_mode"`
	CurrencyDigits    int     `json:"currency_digits"`
	FIFOClose         bool    `json:"fifo_close"`
	Balance           float64 `json:"balance"`
	Credit            float64 `json:"credit"`
	Profit            float64 `json:"profit"`
	Equity            float64 `json:"equity"`
	Margin            float64 `json:"margin"`
	MarginFree        float64 `json:"margin_free"`
	MarginLevel       float64 `json:"margin_level"`
	MarginSOCall      float64 `json:"margin_so_call"`
	MarginSOSO        float64 `json:"margin_so_so"`
	MarginInitial     float64 `json:"margin_initial"`
	MarginMaintenance float64 `json:"margin_maintenance"`
	Assets            float64 `json:"assets"`
	Liabilities       float64 `json:"liabilities"`
	CommissionBlocked float64 `json:"commission_blocked"`
	Name              string  `json:"name"`
	Server            string  `json:"server"`
	Currency          string  `json:"currency"`
	Company           string  `json:"company"`
}

// TradeModeName returns "demo", "contest", "real", or "unknown(N)" for printing.
// Used by the safety layer to skip live-mode checks on demo accounts.
func (a AccountInfo) TradeModeName() string {
	switch a.TradeMode {
	case 0:
		return "demo"
	case 1:
		return "contest"
	case 2:
		return "real"
	default:
		return "unknown"
	}
}

// IsLive reports whether the account requires live-mode safety enforcement.
func (a AccountInfo) IsLive() bool { return a.TradeMode == 2 }

// TerminalInfo mirrors mt5.terminal_info()._asdict().
type TerminalInfo struct {
	CommunityAccount     bool    `json:"community_account"`
	CommunityConnection  bool    `json:"community_connection"`
	Connected            bool    `json:"connected"`
	DLLsAllowed          bool    `json:"dlls_allowed"`
	TradeAllowed         bool    `json:"trade_allowed"`
	TradeAPIDisabled     bool    `json:"tradeapi_disabled"`
	EmailEnabled         bool    `json:"email_enabled"`
	FTPEnabled           bool    `json:"ftp_enabled"`
	NotificationsEnabled bool    `json:"notifications_enabled"`
	MQID                 bool    `json:"mqid"`
	Build                int     `json:"build"`
	MaxBars              int     `json:"maxbars"`
	Codepage             int     `json:"codepage"`
	PingLast             int     `json:"ping_last"`
	CommunityBalance     float64 `json:"community_balance"`
	Retransmission       float64 `json:"retransmission"`
	Company              string  `json:"company"`
	Name                 string  `json:"name"`
	Language             string  `json:"language"`
	Path                 string  `json:"path"`
	DataPath             string  `json:"data_path"`
	CommondataPath       string  `json:"commondata_path"`
}

// InitializeOptions mirrors the kwargs to mt5.initialize().
// All fields are optional; pass an empty struct to connect to a running terminal.
type InitializeOptions struct {
	Path     string `json:"path,omitempty"` // path to terminal64.exe
	Login    int64  `json:"login,omitempty"`
	Password string `json:"password,omitempty"`
	Server   string `json:"server,omitempty"`
	Timeout  int    `json:"timeout,omitempty"` // ms
	Portable bool   `json:"portable,omitempty"`
}

// LoginOptions mirrors the kwargs to mt5.login().
type LoginOptions struct {
	Login    int64  `json:"login"`
	Password string `json:"password"`
	Server   string `json:"server"`
	Timeout  int    `json:"timeout,omitempty"`
}

// ── high-level wrappers ──────────────────────────────────────────────────────

// Initialize connects to a running MT5 terminal (or starts one if Path set).
func (b *Bridge) Initialize(opts InitializeOptions) error {
	return b.Call("initialize", opts, nil)
}

// Login switches the connected terminal to a different broker account.
func (b *Bridge) Login(opts LoginOptions) error {
	return b.Call("login", opts, nil)
}

// Shutdown disconnects the bridge from the terminal (terminal keeps running).
func (b *Bridge) Shutdown() error {
	return b.Call("shutdown", nil, nil)
}

// AccountInfo returns the currently connected account.
func (b *Bridge) AccountInfo() (*AccountInfo, error) {
	var a AccountInfo
	if err := b.Call("account_info", nil, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// TerminalInfo returns info about the connected terminal process.
func (b *Bridge) TerminalInfo() (*TerminalInfo, error) {
	var t TerminalInfo
	if err := b.Call("terminal_info", nil, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// Version returns the MT5 build + release date as reported by the package.
// Returns a 3-element slice: [build, release_date_int, release_str].
func (b *Bridge) Version() ([]any, error) {
	var v []any
	if err := b.Call("version", nil, &v); err != nil {
		return nil, err
	}
	return v, nil
}
