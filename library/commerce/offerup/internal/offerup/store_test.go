package offerup

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := OpenStore(filepath.Join(t.TempDir(), "offerup-test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	return st
}

func TestRecordSearchNewCountAndListings(t *testing.T) {
	st := newTestStore(t)
	key := "iphone@zip:98101"

	n, err := st.RecordSearch(key, []Listing{
		{ListingID: "a", Title: "iPhone 12", Price: 300},
		{ListingID: "b", Title: "iPhone SE", Price: 150},
	})
	require.NoError(t, err)
	assert.Equal(t, 2, n, "both listings are first-seen")

	// Re-recording the same listings yields zero new.
	n, err = st.RecordSearch(key, []Listing{{ListingID: "a", Title: "iPhone 12", Price: 300}})
	require.NoError(t, err)
	assert.Equal(t, 0, n)

	got, err := st.Listings(key)
	require.NoError(t, err)
	assert.Len(t, got, 2)

	// A different location key does not collide.
	other, err := st.Listings("iphone@zip:85001")
	require.NoError(t, err)
	assert.Empty(t, other)
}

func TestNewSinceWindow(t *testing.T) {
	st := newTestStore(t)
	key := "couch@default"
	_, err := st.RecordSearch(key, []Listing{{ListingID: "x", Title: "Couch", Price: 100}})
	require.NoError(t, err)

	// Cutoff in the future -> nothing is "new since then".
	fresh, err := st.NewSince(key, time.Now().Add(time.Hour))
	require.NoError(t, err)
	assert.Empty(t, fresh)

	// Cutoff in the past -> the just-recorded listing is new.
	fresh, err = st.NewSince(key, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	require.Len(t, fresh, 1)
	assert.Equal(t, "x", fresh[0].ListingID)
}

func TestDropsDetection(t *testing.T) {
	st := newTestStore(t)
	key := "tv@zip:98101"
	// First observation at 500.
	_, err := st.RecordSearch(key, []Listing{{ListingID: "tv1", Title: "OLED TV", Price: 500}})
	require.NoError(t, err)

	// No drop yet (single observation).
	drops, err := st.Drops(key, time.Now().Add(-24*time.Hour))
	require.NoError(t, err)
	assert.Empty(t, drops)

	// Second observation: price fell to 400.
	_, err = st.RecordSearch(key, []Listing{{ListingID: "tv1", Title: "OLED TV", Price: 400}})
	require.NoError(t, err)

	drops, err = st.Drops(key, time.Now().Add(-24*time.Hour))
	require.NoError(t, err)
	require.Len(t, drops, 1)
	assert.Equal(t, "tv1", drops[0].ListingID)
	assert.Equal(t, 500.0, drops[0].PriorPrice)
	assert.Equal(t, 400.0, drops[0].CurrentPrice)
	assert.Equal(t, 100.0, drops[0].DropAmount)
	assert.Equal(t, 20.0, drops[0].DropPercent)
}

func TestRecordDetailSellerAndInventory(t *testing.T) {
	st := newTestStore(t)
	require.NoError(t, st.RecordDetail(&ListingDetail{
		Listing: Listing{ListingID: "d1", Title: "Drill", Price: 80},
		OwnerID: "seller-9",
	}))
	require.NoError(t, st.RecordDetail(&ListingDetail{
		Listing: Listing{ListingID: "d2", Title: "Saw", Price: 120},
		OwnerID: "seller-9",
	}))
	require.NoError(t, st.RecordSeller(&Seller{ID: "seller-9", Name: "Bob", PrimaryBadge: "BUSINESS", IsBusinessAccount: true}))

	inv, err := st.SellerInventory("seller-9")
	require.NoError(t, err)
	require.Len(t, inv, 2)
	assert.Equal(t, 100.0, Median(inv))

	sel, err := st.Seller("seller-9")
	require.NoError(t, err)
	require.NotNil(t, sel)
	assert.Equal(t, "Bob", sel.Name)
	assert.True(t, sel.IsBusinessAccount)

	// Unknown seller -> nil, no error.
	none, err := st.Seller("ghost")
	require.NoError(t, err)
	assert.Nil(t, none)
}
