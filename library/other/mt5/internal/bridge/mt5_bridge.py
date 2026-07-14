"""mt5_bridge.py — Python subprocess that fronts the MetaTrader5 package.

Protocol: line-delimited JSON-RPC over stdin/stdout.

  Request  → {"id": 1, "method": "account_info", "params": {}}
  Response ← {"id": 1, "result": {...}}
           ← {"id": 1, "error": {"code": "STR", "message": "..."}}

Error codes (mirror Go's bridge.Err* sentinels):
  NOT_LOGGED_IN          mt5.account_info() returned None
  TERMINAL_DOWN          mt5.initialize() returned False
  BROKER_REJECTED        retcode != TRADE_RETCODE_DONE
  RATE_LIMITED           detected via consecutive timeouts
  PYTHON_MT5_MISSING     `import MetaTrader5` failed at startup
  INVALID_PARAMS         malformed params dict
  INTERNAL               any uncaught exception (also logged to stderr)

Functions covered (full MetaTrader5 API surface — Phase 1 fills in the bodies):
  initialize, login, shutdown, last_error, version,
  account_info, terminal_info,
  symbols_total, symbols_get, symbol_info, symbol_info_tick, symbol_select,
  market_book_add, market_book_get, market_book_release,
  copy_rates_from, copy_rates_from_pos, copy_rates_range,
  copy_ticks_from, copy_ticks_range,
  orders_total, orders_get,
  order_calc_margin, order_calc_profit, order_check, order_send,
  positions_total, positions_get,
  history_orders_total, history_orders_get,
  history_deals_total, history_deals_get.

Run standalone for a quick smoke test:
  py -3 mt5_bridge.py --self-test
"""

from __future__ import annotations

import json
import sys
import traceback
from typing import Any, Callable, Dict

# Lazy import so --self-test works without MetaTrader5 installed for syntax checks.
mt5 = None  # type: ignore


def _load_mt5() -> str | None:
    """Return None on success, an error-code string on failure."""
    global mt5
    try:
        import MetaTrader5 as _mt5  # type: ignore
        mt5 = _mt5
        return None
    except ImportError:
        return "PYTHON_MT5_MISSING"


# ── handlers ────────────────────────────────────────────────────────────────
# Each handler takes a params dict, returns a JSON-serializable result, and
# either raises BridgeError on a known failure or lets the dispatcher turn
# unknown exceptions into INTERNAL.

class BridgeError(Exception):
    def __init__(self, code: str, message: str):
        super().__init__(message)
        self.code = code
        self.message = message


def _to_dict(obj: Any) -> Any:
    """Convert any MetaTrader5 result (named tuple, tuple of named tuples,
    numpy structured array, numpy scalar) into a JSON-serializable form."""
    if obj is None:
        return None
    # numpy: copy_rates_* and copy_ticks_* return structured ndarrays.
    try:
        import numpy as np  # local import: bridge runs without numpy too
        if isinstance(obj, np.ndarray):
            if obj.dtype.names:
                names = obj.dtype.names
                return [dict(zip(names, [_scalar(v) for v in row])) for row in obj.tolist()]
            return [_scalar(v) for v in obj.tolist()]
        if isinstance(obj, np.generic):
            return obj.item()
    except ImportError:
        pass
    if hasattr(obj, "_asdict"):
        return {k: _scalar(v) for k, v in obj._asdict().items()}
    if isinstance(obj, (list, tuple)):
        return [_to_dict(x) for x in obj]
    return obj


def _scalar(v: Any) -> Any:
    """Coerce numpy scalar types to Python natives."""
    try:
        import numpy as np
        if isinstance(v, np.generic):
            return v.item()
    except ImportError:
        pass
    return v


def h_initialize(params: dict) -> Any:
    ok = mt5.initialize(**params)
    if not ok:
        raise BridgeError("TERMINAL_DOWN", "mt5.initialize() returned False — terminal not running?")
    return {"ok": True}


def h_login(params: dict) -> Any:
    ok = mt5.login(**params)
    if not ok:
        raise BridgeError("NOT_LOGGED_IN", "mt5.login() failed — check account/server/password")
    return {"ok": True}


def h_shutdown(params: dict) -> Any:
    mt5.shutdown()
    return {"ok": True}


def h_last_error(params: dict) -> Any:
    return list(mt5.last_error())


def h_version(params: dict) -> Any:
    return list(mt5.version())


def h_account_info(params: dict) -> Any:
    info = mt5.account_info()
    if info is None:
        raise BridgeError("NOT_LOGGED_IN", "account_info() is None")
    return _to_dict(info)


def h_terminal_info(params: dict) -> Any:
    info = mt5.terminal_info()
    if info is None:
        raise BridgeError("TERMINAL_DOWN", "terminal_info() is None")
    return _to_dict(info)


# Symbols
def h_symbols_total(params: dict) -> Any: return mt5.symbols_total()
def h_symbols_get(params: dict) -> Any: return _to_dict(mt5.symbols_get(**params))
def h_symbol_info(params: dict) -> Any:
    _ensure_selected(params["symbol"])
    return _to_dict(mt5.symbol_info(params["symbol"]))

def h_symbol_info_tick(params: dict) -> Any:
    _ensure_selected(params["symbol"])
    return _to_dict(mt5.symbol_info_tick(params["symbol"]))

def h_symbol_select(params: dict) -> Any: return {"ok": mt5.symbol_select(params["symbol"], params.get("enable", True))}


# Market book — get implicitly subscribes; broker must support DOM.
def h_market_book_add(params: dict) -> Any: return {"ok": mt5.market_book_add(params["symbol"])}
def h_market_book_release(params: dict) -> Any: return {"ok": mt5.market_book_release(params["symbol"])}
def h_market_book_get(params: dict) -> Any:
    sym = params["symbol"]
    _ensure_selected(sym)
    # market_book_add returning False is broker-dependent: some report "no DOM"
    # this way, others succeed and return depth on the next get. We try both.
    mt5.market_book_add(sym)
    return _to_dict(mt5.market_book_get(sym)) or []


# Bars + ticks — normalize timeframe strings ("M5") and unix-int dates to MT5 types.

TF_MAP = {
    "M1":  "TIMEFRAME_M1",  "M2":  "TIMEFRAME_M2",  "M3":  "TIMEFRAME_M3",
    "M4":  "TIMEFRAME_M4",  "M5":  "TIMEFRAME_M5",  "M6":  "TIMEFRAME_M6",
    "M10": "TIMEFRAME_M10", "M12": "TIMEFRAME_M12", "M15": "TIMEFRAME_M15",
    "M20": "TIMEFRAME_M20", "M30": "TIMEFRAME_M30",
    "H1": "TIMEFRAME_H1", "H2": "TIMEFRAME_H2", "H3": "TIMEFRAME_H3",
    "H4": "TIMEFRAME_H4", "H6": "TIMEFRAME_H6", "H8": "TIMEFRAME_H8",
    "H12": "TIMEFRAME_H12",
    "D1": "TIMEFRAME_D1", "W1": "TIMEFRAME_W1", "MN1": "TIMEFRAME_MN1",
}

def _tf(v):
    if isinstance(v, str):
        name = TF_MAP.get(v.upper())
        if name is None:
            raise BridgeError("INVALID_PARAMS", f"unknown timeframe {v!r}")
        return getattr(mt5, name)
    return v

def _dt(v):
    """Accept unix seconds (int/float) or ISO string. Return datetime."""
    import datetime as _dt
    if isinstance(v, (int, float)):
        return _dt.datetime.fromtimestamp(float(v), _dt.timezone.utc)
    if isinstance(v, str):
        # support both 'YYYY-MM-DD' and 'YYYY-MM-DD HH:MM:SS'
        for fmt in ("%Y-%m-%dT%H:%M:%S", "%Y-%m-%d %H:%M:%S", "%Y-%m-%d"):
            try:
                return _dt.datetime.strptime(v, fmt)
            except ValueError:
                continue
        raise BridgeError("INVALID_PARAMS", f"unparseable date {v!r}")
    return v

def _bars_params(p):
    p = dict(p)
    if "timeframe" in p: p["timeframe"] = _tf(p["timeframe"])
    if "date_from" in p: p["date_from"] = _dt(p["date_from"])
    if "date_to"   in p: p["date_to"]   = _dt(p["date_to"])
    return p

def _ticks_params(p):
    p = dict(p)
    if "date_from" in p: p["date_from"] = _dt(p["date_from"])
    if "date_to"   in p: p["date_to"]   = _dt(p["date_to"])
    if "flags"     in p and isinstance(p["flags"], str):
        # accept "all" | "info" | "trade" shortcuts
        m = {"all": mt5.COPY_TICKS_ALL, "info": mt5.COPY_TICKS_INFO, "trade": mt5.COPY_TICKS_TRADE}
        p["flags"] = m.get(p["flags"].lower(), p["flags"])
    return p

# MT5's C bindings are positional-only for the bars/ticks copy family.
# We accept named params on the wire and unpack in the documented order.

def _ensure_selected(symbol: str) -> None:
    """copy_rates_range / copy_ticks_range return nothing for unselected
    symbols. We Select-on-the-fly; harmless if the symbol is already shown."""
    if not mt5.symbol_select(symbol, True):
        err = mt5.last_error()
        raise BridgeError("INVALID_PARAMS", f"symbol_select({symbol}) failed: {err}")

def h_copy_rates_from(params: dict) -> Any:
    p = _bars_params(params)
    _ensure_selected(p["symbol"])
    return _to_dict(mt5.copy_rates_from(p["symbol"], p["timeframe"], p["date_from"], p.get("count", 0)))

def h_copy_rates_from_pos(params: dict) -> Any:
    p = _bars_params(params)
    _ensure_selected(p["symbol"])
    return _to_dict(mt5.copy_rates_from_pos(p["symbol"], p["timeframe"], p.get("start_pos", 0), p.get("count", 0)))

def h_copy_rates_range(params: dict) -> Any:
    p = _bars_params(params)
    _ensure_selected(p["symbol"])
    res = mt5.copy_rates_range(p["symbol"], p["timeframe"], p["date_from"], p["date_to"])
    if res is None:
        # None is MT5's failure signal (empty ranges come back as a 0-row
        # array); as null it would make a failed sync report 0 rows inserted.
        raise BridgeError("TERMINAL_DOWN", f"copy_rates_range() returned None: {mt5.last_error()}")
    return _to_dict(res)

def h_copy_ticks_from(params: dict) -> Any:
    p = _ticks_params(params)
    _ensure_selected(p["symbol"])
    return _to_dict(mt5.copy_ticks_from(p["symbol"], p["date_from"], p.get("count", 0), p.get("flags", mt5.COPY_TICKS_ALL)))

def h_copy_ticks_range(params: dict) -> Any:
    p = _ticks_params(params)
    _ensure_selected(p["symbol"])
    res = mt5.copy_ticks_range(p["symbol"], p["date_from"], p["date_to"], p.get("flags", mt5.COPY_TICKS_ALL))
    if res is None:
        # See h_copy_rates_range: None is failure, not an empty range.
        raise BridgeError("TERMINAL_DOWN", f"copy_ticks_range() returned None: {mt5.last_error()}")
    return _to_dict(res)


# Orders / positions / history
def h_orders_total(params: dict) -> Any: return mt5.orders_total()
def h_orders_get(params: dict) -> Any:
    # orders_get() returns None on failure and () when there are genuinely no
    # pending orders; None must not flow through as an empty snapshot.
    res = mt5.orders_get(**params)
    if res is None:
        raise BridgeError("TERMINAL_DOWN", f"orders_get() returned None: {mt5.last_error()}")
    return _to_dict(res)
def h_order_calc_margin(params: dict) -> Any:
    _ensure_selected(params["symbol"])
    return mt5.order_calc_margin(
        params["action"], params["symbol"], params["volume"], params["price"],
    )

def h_order_calc_profit(params: dict) -> Any:
    _ensure_selected(params["symbol"])
    return mt5.order_calc_profit(
        params["action"], params["symbol"], params["volume"],
        params["price_open"], params["price_close"],
    )
def h_order_check(params: dict) -> Any: return _to_dict(mt5.order_check(params["request"]))
def h_order_send(params: dict) -> Any:
    # Always return the result — Go inspects retcode and decides exit code so
    # we don't drop the broker's structured response on a rejection.
    return _to_dict(mt5.order_send(params["request"]))


def h_positions_total(params: dict) -> Any: return mt5.positions_total()
def h_positions_get(params: dict) -> Any:
    # positions_get() returns None on failure and () when flat — see
    # h_orders_get for why None must not flow through as an empty snapshot.
    res = mt5.positions_get(**params)
    if res is None:
        raise BridgeError("TERMINAL_DOWN", f"positions_get() returned None: {mt5.last_error()}")
    return _to_dict(res)
def _history_params(p):
    p = dict(p)
    if "date_from" in p: p["date_from"] = _dt(p["date_from"])
    if "date_to"   in p: p["date_to"]   = _dt(p["date_to"])
    return p

# history_* are positional (date_from, date_to) plus optional group/ticket/position kwargs.
# We pass date_from/date_to positionally and any remaining keys as kwargs.

def _history_args(params: dict):
    p = _history_params(params)
    dfrom, dto = p.pop("date_from", None), p.pop("date_to", None)
    return dfrom, dto, p

def h_history_orders_total(params: dict) -> Any:
    dfrom, dto, rest = _history_args(params)
    return mt5.history_orders_total(dfrom, dto, **rest)

def h_history_orders_get(params: dict) -> Any:
    dfrom, dto, rest = _history_args(params)
    return _to_dict(mt5.history_orders_get(dfrom, dto, **rest))

def h_history_deals_total(params: dict) -> Any:
    dfrom, dto, rest = _history_args(params)
    return mt5.history_deals_total(dfrom, dto, **rest)

def h_history_deals_get(params: dict) -> Any:
    dfrom, dto, rest = _history_args(params)
    return _to_dict(mt5.history_deals_get(dfrom, dto, **rest))


HANDLERS: Dict[str, Callable[[dict], Any]] = {
    "initialize": h_initialize, "login": h_login, "shutdown": h_shutdown,
    "last_error": h_last_error, "version": h_version,
    "account_info": h_account_info, "terminal_info": h_terminal_info,
    "symbols_total": h_symbols_total, "symbols_get": h_symbols_get,
    "symbol_info": h_symbol_info, "symbol_info_tick": h_symbol_info_tick,
    "symbol_select": h_symbol_select,
    "market_book_add": h_market_book_add, "market_book_get": h_market_book_get,
    "market_book_release": h_market_book_release,
    "copy_rates_from": h_copy_rates_from, "copy_rates_from_pos": h_copy_rates_from_pos,
    "copy_rates_range": h_copy_rates_range,
    "copy_ticks_from": h_copy_ticks_from, "copy_ticks_range": h_copy_ticks_range,
    "orders_total": h_orders_total, "orders_get": h_orders_get,
    "order_calc_margin": h_order_calc_margin, "order_calc_profit": h_order_calc_profit,
    "order_check": h_order_check, "order_send": h_order_send,
    "positions_total": h_positions_total, "positions_get": h_positions_get,
    "history_orders_total": h_history_orders_total,
    "history_orders_get": h_history_orders_get,
    "history_deals_total": h_history_deals_total,
    "history_deals_get": h_history_deals_get,
}


# ── dispatcher ──────────────────────────────────────────────────────────────


def _serve_one(line: str) -> str:
    """Process one JSON request line; return one JSON response line."""
    try:
        req = json.loads(line)
    except json.JSONDecodeError as e:
        return json.dumps({"id": None, "error": {"code": "INVALID_PARAMS", "message": f"bad json: {e}"}})
    rid = req.get("id")
    method = req.get("method", "")
    params = req.get("params") or {}
    handler = HANDLERS.get(method)
    if handler is None:
        return json.dumps({"id": rid, "error": {"code": "INVALID_PARAMS", "message": f"unknown method {method!r}"}})
    if mt5 is None:
        return json.dumps({"id": rid, "error": {"code": "PYTHON_MT5_MISSING", "message": "MetaTrader5 package not loaded"}})
    try:
        result = handler(params)
        return json.dumps({"id": rid, "result": result}, default=str)
    except BridgeError as e:
        return json.dumps({"id": rid, "error": {"code": e.code, "message": e.message}})
    except Exception as e:  # noqa: BLE001
        traceback.print_exc(file=sys.stderr)
        return json.dumps({"id": rid, "error": {"code": "INTERNAL", "message": str(e)}})


def serve() -> None:
    """Read JSON-RPC requests from stdin until EOF, write responses to stdout."""
    err = _load_mt5()
    if err:
        # Keep the loop alive so the caller can read the error per-request and
        # decide whether to give up. doctor pings this and surfaces the code.
        pass
    for raw in sys.stdin:
        line = raw.strip()
        if not line:
            continue
        sys.stdout.write(_serve_one(line) + "\n")
        sys.stdout.flush()


def self_test() -> int:
    """Print loadability diagnostics and exit."""
    err = _load_mt5()
    print(json.dumps({
        "python": sys.version.split()[0],
        "platform": sys.platform,
        "mt5_loaded": err is None,
        "mt5_error": err,
        "handlers": sorted(HANDLERS.keys()),
        "handler_count": len(HANDLERS),
    }, indent=2))
    return 0 if err is None else 1


if __name__ == "__main__":
    if "--self-test" in sys.argv:
        sys.exit(self_test())
    serve()
