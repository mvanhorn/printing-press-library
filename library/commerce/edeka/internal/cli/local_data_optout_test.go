package cli

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/edeka/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/commerce/edeka/internal/edeka"
)

// TestRecordPricesHonoursNoLocalData: with EDEKA_NO_LOCAL_DATA=1 a search or
// offers fetch must not create or write the local price store.
func TestRecordPricesHonoursNoLocalData(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv(noLocalDataEnvVar, "1")
	recordPrices(context.Background(), []edeka.Product{{ID: "1000001MXST", Title: "Test Milch", PriceCents: 119}}, "test-market.edeka.shop", "search")
	if _, err := os.Stat(defaultDBPath("edeka-pp-cli")); !os.IsNotExist(err) {
		t.Fatalf("price store written despite %s=1 (stat err %v)", noLocalDataEnvVar, err)
	}
}

// TestRecordPricesWritesByDefault is the control case for the opt-out test.
func TestRecordPricesWritesByDefault(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv(noLocalDataEnvVar, "")
	recordPrices(context.Background(), []edeka.Product{{ID: "1000001MXST", Title: "Test Milch", PriceCents: 119}}, "test-market.edeka.shop", "search")
	if _, err := os.Stat(defaultDBPath("edeka-pp-cli")); err != nil {
		t.Fatalf("price store not written: %v", err)
	}
}

// TestPriceHistoryNoLocalDataCreatesNoStore: reading price history while
// local data is off must not create a store on disk.
func TestPriceHistoryNoLocalDataCreatesNoStore(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv(noLocalDataEnvVar, "1")
	cmd := RootCmd()
	cmd.SetArgs([]string{"price", "history", "milch", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	_ = cmd.Execute()
	if _, err := os.Stat(defaultDBPath("edeka-pp-cli")); !os.IsNotExist(err) {
		t.Fatalf("price history created a store despite %s=1 (stat err %v)", noLocalDataEnvVar, err)
	}
	if !bytes.Contains(out.Bytes(), []byte(noLocalDataEnvVar)) {
		t.Fatalf("output does not explain the opt-out:\n%s", out.String())
	}
}
