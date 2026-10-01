// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package actual_test

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual/actualtest"
)

var benchMerchants = []string{"amazon", "kroger", "walmart", "target", "costco", "chipotle", "starbucks", "netflix",
	"shell", "home depot", "uber", "lyft", "apple", "spotify", "whole foods", "trader joes", "safeway", "cvs",
	"walgreens", "mcdonalds", "transfer to savings", "transfer from savings", "zelle to", "zelle from"}

// synthPayees returns n deterministic payee names: merchant stems with
// spelling noise, store numbers, suffixes and random filler words.
func synthPayees(n int) []actual.Payee {
	rng := rand.New(rand.NewSource(1))
	suffix := []string{"", " INC", " #1234", ".com", " MKTPLACE", " fuel", " store", " online", "*AB12", " LLC"}
	letters := "abcdefghijklmnopqrstuvwxyz"
	out := make([]actual.Payee, n)
	for i := range out {
		var name string
		if rng.Intn(3) == 0 {
			m := benchMerchants[rng.Intn(len(benchMerchants))]
			b := []byte(m)
			if rng.Intn(2) == 0 && len(b) > 3 {
				k := rng.Intn(len(b))
				b[k] = letters[rng.Intn(len(letters))]
			}
			name = strings.ToUpper(string(b)) + suffix[rng.Intn(len(suffix))]
		} else {
			words := 1 + rng.Intn(3)
			var parts []string
			for w := 0; w < words; w++ {
				l := 3 + rng.Intn(8)
				b := make([]byte, l)
				for j := range b {
					b[j] = letters[rng.Intn(len(letters))]
				}
				parts = append(parts, string(b))
			}
			name = strings.Join(parts, " ") + suffix[rng.Intn(len(suffix))]
		}
		out[i] = actual.Payee{ID: fmt.Sprintf("p%05d", i), Name: name, TxnCount: rng.Intn(200)}
	}
	return out
}

func BenchmarkClusterPayees(b *testing.B) {
	payees := synthPayees(2000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		actual.ClusterPayees(payees, 0.85)
	}
}

// BenchmarkAuditRules runs AuditRules over 300 rules and 50k transactions.
func BenchmarkAuditRules(b *testing.B) {
	path := filepath.Join(b.TempDir(), "db.sqlite")
	if err := actualtest.Build(path); err != nil {
		b.Fatal(err)
	}
	w, err := sql.Open("sqlite", path)
	if err != nil {
		b.Fatal(err)
	}
	rng := rand.New(rand.NewSource(2))
	tx, err := w.Begin()
	if err != nil {
		b.Fatal(err)
	}
	payeeIDs := []string{"pay-kroger", "pay-amazon", "pay-amazon2", "pay-chipotle", "pay-netflix", "pay-employer"}
	descs := []string{"KROGER #123 CINCINNATI", "AMZN Mktp US*2K4", "Chipotle 0921", "NETFLIX.COM", "Starbucks Store 55", "Shell Oil 1234"}
	ins, err := tx.Prepare(`INSERT INTO transactions (id,acct,category,amount,description,notes,imported_description,date,sort_order,cleared) VALUES (?,?,?,?,?,?,?,?,?,1)`)
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 50000; i++ {
		d := 20250101 + rng.Intn(12)*100 + rng.Intn(28)
		if _, err := ins.Exec(fmt.Sprintf("bt-%06d", i), "acct-card", "cat-groceries", -int64(rng.Intn(20000)),
			payeeIDs[rng.Intn(len(payeeIDs))], fmt.Sprintf("Note %d Weekly", rng.Intn(50)), descs[rng.Intn(len(descs))], d, i); err != nil {
			b.Fatal(err)
		}
	}
	ops := []string{
		`{"op":"contains","field":"imported_payee","value":"%s","type":"string"}`,
		`{"op":"is","field":"notes","value":"%s","type":"string"}`,
		`{"op":"oneOf","field":"imported_payee","value":["%s","x"],"type":"string"}`,
		`{"op":"doesNotContain","field":"notes","value":"%s","type":"string"}`,
	}
	words := []string{"kroger", "Amazon", "STARBUCKS", "note 7 weekly", "shell", "netflix.com", "zzz"}
	rins, err := tx.Prepare(`INSERT INTO rules (id,stage,conditions_op,conditions,actions) VALUES (?,NULL,?,?,'[{"op":"set","field":"category","value":"cat-dining","type":"id"}]')`)
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 300; i++ {
		var conds []string
		for c := 0; c < 1+rng.Intn(3); c++ {
			conds = append(conds, fmt.Sprintf(ops[rng.Intn(len(ops))], words[rng.Intn(len(words))]))
		}
		if rng.Intn(4) == 0 {
			conds = append(conds, fmt.Sprintf(`{"op":"is","field":"payee","value":"%s","type":"id"}`, payeeIDs[rng.Intn(len(payeeIDs))]))
		}
		op := "and"
		if rng.Intn(3) == 0 {
			op = "or"
		}
		if _, err := rins.Exec(fmt.Sprintf("brule-%03d", i), op, "["+strings.Join(conds, ",")+"]"); err != nil {
			b.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		b.Fatal(err)
	}
	w.Close()
	db, err := actual.OpenReadOnly(path)
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	l := actual.NewLedger(db)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := l.AuditRules(context.Background(), actual.AuditRulesOptions{}); err != nil {
			b.Fatal(err)
		}
	}
}
