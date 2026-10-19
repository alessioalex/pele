package common

import "strings"

// pairTildes pairs tildes from the outside in: the first with the last, the second with the
// one before it.
func pairTildes(t string) string {
	count := strings.Count(t, "~")
	if count <= 1 {
		return t
	}
	lead := count%2 != 0 && strings.HasPrefix(t, "~")
	chars := []byte(t)
	var at []int
	start := 0
	if lead {
		start = 1
	}
	for i := start; i < len(chars); i++ {
		if chars[i] == '~' {
			at = append(at, i)
		}
	}
	for lo, hi := 0, len(at)-1; lo < hi; lo, hi = lo+1, hi-1 {
		chars[at[lo]] = '<'
		chars[at[hi]] = '>'
	}
	return string(chars)
}

// ParseGenericTypes turns `List~int~` into `List<int>`, with the quirks of Mermaid's
// parseGenericTypes around commas.
func ParseGenericTypes(input string) string {
	if !strings.Contains(input, "~") {
		return input
	}
	// input.split(/(,)/) keeps the commas as their own pieces.
	var sets []string
	for _, part := range strings.Split(input, ",") {
		if len(sets) > 0 {
			sets = append(sets, ",")
		}
		sets = append(sets, part)
	}
	var output []string
	for i := 0; i < len(sets); i++ {
		set := sets[i]
		if set == "," && i > 0 && i+1 < len(sets) && strings.Count(sets[i-1], "~") == 1 && strings.Count(sets[i+1], "~") == 1 {
			set = sets[i-1] + "," + sets[i+1]
			i++
			output = output[:len(output)-1]
		}
		output = append(output, pairTildes(set))
	}
	return strings.Join(output, "")
}
