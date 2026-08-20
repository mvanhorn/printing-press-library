package cli

// jaroWinkler returns similarity in [0,1]. Standard Jaro with the Winkler
// common-prefix boost (p=0.1, max 4 chars). Used to cluster near-duplicate
// canonical names that Layer-A folding did not merge (typos, minor edits).
func jaroWinkler(a, b string) float64 {
	if a == b {
		return 1
	}
	ra, rb := []rune(a), []rune(b)
	la, lb := len(ra), len(rb)
	if la == 0 || lb == 0 {
		return 0
	}
	matchDist := max(la, lb)/2 - 1
	if matchDist < 0 {
		matchDist = 0
	}
	aMatch := make([]bool, la)
	bMatch := make([]bool, lb)
	matches := 0
	for i := 0; i < la; i++ {
		lo := max(0, i-matchDist)
		hi := min(lb-1, i+matchDist)
		for j := lo; j <= hi; j++ {
			if bMatch[j] || ra[i] != rb[j] {
				continue
			}
			aMatch[i], bMatch[j] = true, true
			matches++
			break
		}
	}
	if matches == 0 {
		return 0
	}
	// Count transpositions.
	trans := 0
	k := 0
	for i := 0; i < la; i++ {
		if !aMatch[i] {
			continue
		}
		for !bMatch[k] {
			k++
		}
		if ra[i] != rb[k] {
			trans++
		}
		k++
	}
	m := float64(matches)
	jaro := (m/float64(la) + m/float64(lb) + (m-float64(trans)/2)/m) / 3
	// Winkler prefix boost.
	prefix := 0
	for prefix < min(4, min(la, lb)) && ra[prefix] == rb[prefix] {
		prefix++
	}
	return jaro + float64(prefix)*0.1*(1-jaro)
}

// clusterNames groups names whose pairwise jaroWinkler >= threshold using
// greedy single-link assignment. Returned clusters partition the input.
func clusterNames(names []string, threshold float64) [][]string {
	assigned := make([]bool, len(names))
	var clusters [][]string
	for i := range names {
		if assigned[i] {
			continue
		}
		cluster := []string{names[i]}
		assigned[i] = true
		for j := i + 1; j < len(names); j++ {
			if assigned[j] {
				continue
			}
			if jaroWinkler(names[i], names[j]) >= threshold {
				cluster = append(cluster, names[j])
				assigned[j] = true
			}
		}
		clusters = append(clusters, cluster)
	}
	return clusters
}
