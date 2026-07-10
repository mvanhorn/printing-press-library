package nsdl

// ParseGenericRecords extracts the largest table on a page into
// column-name -> cell-value maps. Used for report pages (AUC, trades,
// registry) whose schema is a flat GridView without the deep grouped-header
// structure of the net-investment series, where positional parsing would be
// brittle to reorder but composite column naming is sufficient.
func ParseGenericRecords(body []byte) ([]map[string]string, error) {
	t, err := ExtractLargestTable(body)
	if err != nil {
		return nil, err
	}
	records := make([]map[string]string, 0, len(t.Rows))
	for _, row := range t.Rows {
		rec := make(map[string]string, len(t.Columns))
		nonEmpty := false
		for i, col := range t.Columns {
			if i >= len(row) {
				continue
			}
			v := row[i]
			if v != "" {
				nonEmpty = true
			}
			key := col
			if key == "" {
				key = itoa(i)
			}
			rec[key] = v
		}
		if nonEmpty {
			records = append(records, rec)
		}
	}
	return records, nil
}
