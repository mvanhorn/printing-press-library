package adsanalytics

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type ProductCost struct {
	Name         string  `toml:"name" json:"name,omitempty"`
	COGS         float64 `toml:"cogs" json:"cogs"`
	SellingPrice float64 `toml:"selling_price" json:"selling_price"`
}

func DefaultCOGSPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "amazon-ads-pp-cli", "cogs.toml")
}

func LoadCOGS(path string) (map[string]ProductCost, error) {
	if path == "" {
		path = DefaultCOGSPath()
	}
	if path == "" {
		return nil, fmt.Errorf("resolving COGS path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading COGS file %s: %w", path, err)
	}
	items := map[string]ProductCost{}
	if err := toml.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("parsing COGS file %s: %w", path, err)
	}
	return items, nil
}

func ResolveProductCost(items map[string]ProductCost, asin string, price, cogs float64) (ProductCost, error) {
	asin = strings.TrimSpace(asin)
	item := ProductCost{SellingPrice: price, COGS: cogs}
	if asin != "" {
		if configured, ok := items[asin]; ok {
			item = configured
		}
	}
	if price > 0 {
		item.SellingPrice = price
	}
	if cogs > 0 {
		item.COGS = cogs
	}
	if item.SellingPrice <= 0 {
		return item, fmt.Errorf("selling price must be greater than zero")
	}
	if item.COGS < 0 {
		return item, fmt.Errorf("COGS must not be negative")
	}
	return item, nil
}

func BreakEvenACOS(price, cogs, feePercent float64) (float64, error) {
	if price <= 0 {
		return 0, fmt.Errorf("selling price must be greater than zero")
	}
	if cogs < 0 {
		return 0, fmt.Errorf("COGS must not be negative")
	}
	if feePercent < 0 || feePercent > 100 {
		return 0, fmt.Errorf("fee percent must be between 0 and 100")
	}
	fees := price * (feePercent / 100)
	return (price - cogs - fees) / price, nil
}

func TrueProfit(price, cogs, feePercent, adSpend float64) (float64, error) {
	breakEven, err := BreakEvenACOS(price, cogs, feePercent)
	if err != nil {
		return 0, err
	}
	marginBeforeAds := price * breakEven
	return marginBeforeAds - adSpend, nil
}

func ACOS(adSpend, adRevenue float64) (float64, error) {
	if adSpend < 0 || adRevenue < 0 {
		return 0, fmt.Errorf("spend and revenue must not be negative")
	}
	if adRevenue == 0 {
		return 0, fmt.Errorf("ad revenue must be greater than zero")
	}
	return adSpend / adRevenue, nil
}

func TACOS(adSpend, totalRevenue float64) (float64, error) {
	if adSpend < 0 || totalRevenue < 0 {
		return 0, fmt.Errorf("spend and revenue must not be negative")
	}
	if totalRevenue == 0 {
		return 0, fmt.Errorf("total revenue must be greater than zero")
	}
	return adSpend / totalRevenue, nil
}
