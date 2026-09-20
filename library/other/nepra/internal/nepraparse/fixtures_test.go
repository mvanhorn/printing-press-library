package nepraparse

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// Fixture files.
//
// full-*.htm.gz are the three published sheet001.htm files verbatim, gzipped
// only so the repository does not carry three ~500 KB blobs. Decompressed
// they are byte-identical to what NEPRA serves, which is what lets the tests
// assert the measured whole-file numbers (row counts, cell counts, census,
// the Sum invariant) rather than approximations of them.
//
// trim-*.htm are hand-trimmed subsets — the complete header band plus a
// chosen set of data rows that between them exercise every cell state,
// colspan-on-data-row shape, thousands separator, hidden-span name join and
// residue cell. They are plain text so a reviewer can read the markup that
// each assertion is about.
const (
	fullFY2324 = "full-fy2023-24.htm.gz"
	fullFY2021 = "full-fy2020-21.htm.gz"
	fullFY1718 = "full-fy2017-18.htm.gz"

	trimFY2324 = "trim-fy2023-24.htm"
	trimFY2021 = "trim-fy2020-21.htm"
	trimFY1718 = "trim-fy2017-18.htm"
)

// fixture returns the raw, undecoded workbook bytes. Nothing in the test
// suite decodes on the way in: ParseWorkbook must handle windows-1252 itself
// because the HTTP response carries no charset.
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	if filepath.Ext(name) != ".gz" {
		return raw
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("opening gzip fixture %s: %v", name, err)
	}
	defer func() { _ = zr.Close() }()
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("decompressing fixture %s: %v", name, err)
	}
	return out
}

// parse is the happy path used by most tests.
func parse(t *testing.T, name, fy string) *Workbook {
	t.Helper()
	w, err := ParseWorkbook(fixture(t, name), fy)
	if err != nil {
		t.Fatalf("ParseWorkbook(%s, %q): %v", name, fy, err)
	}
	return w
}

// yearFixture bundles one full year's measured ground truth. Every number
// here was measured against the live file before the parser was written.
type yearFixture struct {
	name string
	fy   string

	// Encoding.
	bytes     int
	nbspBytes int

	// Structure.
	tableRows     int
	rawCells      int
	physicalWidth int
	// rawCellHistogram maps a raw <td>-per-<tr> count to how many rows have
	// it. The 14-cell rows in FY2023-24 are the header band plus the fifteen
	// data rows carrying a single <td colspan=26>.
	rawCellHistogram map[int]int

	// Rows.
	plants        int
	separatorRows int

	// Census over the monthly block.
	present            int
	notReported        int
	delicensed         int
	decommissioned     int
	measuredZeros      int
	thousandsSeparated int
	notReportedRows    int
	statusRows         int
	capacityStatus     int
	residueCells       int

	// Sum invariant.
	gwhPassed, gwhEligible int
	pctPassed, pctEligible int
}

// fullYears is the measured ground truth for the three published years.
var fullYears = []yearFixture{
	{
		name: fullFY2324, fy: "2023-24",
		bytes: 493187, nbspBytes: 132,
		tableRows: 139, rawCells: 4965, physicalWidth: 39,
		rawCellHistogram: map[int]int{8: 1, 14: 16, 20: 1, 33: 1, 39: 120},
		plants:           133, separatorRows: 2,
		present:         3068,
		notReported:     52,
		delicensed:      312,
		decommissioned:  26,
		measuredZeros:   471,
		notReportedRows: 2,
		statusRows:      13,
		capacityStatus:  0,
		residueCells:    1,
		// 68 comma-bearing cells were measured over all 32 logical columns;
		// the census counts the monthly block plus the two capacity columns,
		// which is the same set.
		thousandsSeparated: 68,
		gwhPassed:          118, gwhEligible: 118,
		pctPassed: 3, pctEligible: 118,
	},
	{
		name: fullFY2021, fy: "2020-21",
		bytes: 455640, nbspBytes: 128,
		tableRows: 114, rawCells: 4479, physicalWidth: 40,
		rawCellHistogram: map[int]int{9: 1, 15: 1, 21: 1, 34: 1, 40: 110},
		plants:           108, separatorRows: 2,
		present:            2730,
		notReported:        78,
		delicensed:         0,
		decommissioned:     0,
		measuredZeros:      540,
		notReportedRows:    3,
		statusRows:         0,
		capacityStatus:     3,
		residueCells:       1,
		thousandsSeparated: 59,
		gwhPassed:          104, gwhEligible: 105,
		pctPassed: 12, pctEligible: 105,
	},
	{
		name: fullFY1718, fy: "2017-18",
		bytes: 426282, nbspBytes: 128,
		tableRows: 114, rawCells: 4479, physicalWidth: 40,
		rawCellHistogram: map[int]int{9: 1, 15: 1, 21: 1, 34: 1, 40: 110},
		plants:           108, separatorRows: 2,
		present:            2522,
		notReported:        286,
		delicensed:         0,
		decommissioned:     0,
		measuredZeros:      437,
		notReportedRows:    11,
		statusRows:         0,
		capacityStatus:     0,
		residueCells:       1,
		thousandsSeparated: 61,
		gwhPassed:          97, gwhEligible: 97,
		pctPassed: 7, pctEligible: 97,
	},
}
