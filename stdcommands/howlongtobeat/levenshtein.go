// Simplest forms of Levenshtein Distance Calculation Functions
// this is case-sensitive

package howlongtobeat

func lowest(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
	} else {
		if b < c {
			return b
		}
	}
	return c
}

func levenshtein(str1, str2 []rune) (int, float64) {
	str1len := len(str1)
	str2len := len(str2)
	distance := make([]int, str1len+1)

	for i := 1; i <= str1len; i++ {
		distance[i] = i
	}
	for j := 1; j <= str2len; j++ {
		distance[0] = j
		lastkey := j - 1
		for i := 1; i <= str1len; i++ {
			oldkey := distance[i]
			var incr int
			if str1[i-1] != str2[j-1] {
				incr = 1
			}

			distance[i] = lowest(distance[i]+1, distance[i-1]+1, lastkey+incr)
			lastkey = oldkey
		}
	}

	levdistance := distance[str1len]
	var similarity float64
	similarity = (float64(str2len) - float64(levdistance)) / float64(str2len)
	return levdistance, similarity
}
