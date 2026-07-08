package sync

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
)

var (
	budgetPattern     = regexp.MustCompile(`(?:^|[,\s])api-usage=(\d+)/(\d+)`)
	ErrBudgetExceeded = errors.New("salesforce api budget exceeded")
)

type Budget struct {
	Used  int
	Limit int
}

func (b Budget) Utilization() float64 {
	if b.Limit <= 0 {
		return 0
	}
	return float64(b.Used) / float64(b.Limit)
}

func ParseBudget(header string) (Budget, bool, error) {
	match := budgetPattern.FindStringSubmatch(header)
	if match == nil {
		return Budget{}, false, nil
	}
	used, err := strconv.Atoi(match[1])
	if err != nil {
		return Budget{}, false, fmt.Errorf("parse api usage used: %w", err)
	}
	limit, err := strconv.Atoi(match[2])
	if err != nil {
		return Budget{}, false, fmt.Errorf("parse api usage limit: %w", err)
	}
	return Budget{Used: used, Limit: limit}, true, nil
}

type Gate struct {
	Budget Budget
	Cutoff float64
}

func NewGate() *Gate {
	return &Gate{Cutoff: 0.80}
}

func (g *Gate) UpdateFromHeader(header string) error {
	budget, ok, err := ParseBudget(header)
	if err != nil || !ok {
		return err
	}
	g.Budget = budget
	return nil
}

func (g *Gate) UpdateFromHeaders(headers http.Header) error {
	if headers == nil {
		return nil
	}
	return g.UpdateFromHeader(headers.Get("Sforce-Limit-Info"))
}

func (g *Gate) Check() error {
	if g == nil {
		return nil
	}
	cutoff := g.Cutoff
	if cutoff <= 0 {
		cutoff = 0.80
	}
	if g.Budget.Limit > 0 && g.Budget.Utilization() >= cutoff {
		return fmt.Errorf("%w: api-usage=%d/%d utilization=%.2f cutoff=%.2f", ErrBudgetExceeded, g.Budget.Used, g.Budget.Limit, g.Budget.Utilization(), cutoff)
	}
	return nil
}
