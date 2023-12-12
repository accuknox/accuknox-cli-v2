package report

type EditAction interface{}

type Keep struct {
	line string
}

type Insert struct {
	line string
}

type Remove struct {
	line string
}

type Frontier struct {
	x       int
	history []EditAction
}

// Myers diff algorithm
// Amazing read: blog.jcoglan.com/2017/02/12/the-myers-diff-algorithm-part-1/ and other parts
// Few more reads: [https://www.nathaniel.ai/myers-diff/] [https://epxx.co/artigos/diff_en.html]
// Complexity: O((N+M)D) where N and M are the lengths of the sequences and D is the number of edits
// Space: O(N+M)
// Reference implementation in Python: https://gist.github.com/adamnew123456/37923cf53f51d6b9af32a539cdfa7cc4
func myersDiff(aLines, bLines []string) []EditAction {
	frontier := make(map[int]Frontier)
	frontier[1] = Frontier{0, []EditAction{}}

	aMax := len(aLines)
	bMax := len(bLines)
	for d := 0; d <= aMax+bMax; d++ {
		for k := -d; k <= d; k += 2 {
			goDown := k == -d || (k != d && frontier[k-1].x < frontier[k+1].x)

			var oldX int
			var history []EditAction

			if goDown {
				oldX = frontier[k+1].x
				history = append([]EditAction{}, frontier[k+1].history...)
			} else {
				oldX = frontier[k-1].x + 1
				history = append([]EditAction{}, frontier[k-1].history...)
			}

			y := oldX - k

			if 1 <= y && y <= bMax && goDown {
				history = append(history, Insert{bLines[y-1]})
			} else if 1 <= oldX && oldX <= aMax {
				history = append(history, Remove{aLines[oldX-1]})
			}

			for oldX < aMax && y < bMax && aLines[oldX] == bLines[y] {
				history = append(history, Keep{aLines[oldX]})
				oldX++
				y++
			}

			if oldX >= aMax && y >= bMax {
				return history
			} else {
				frontier[k] = Frontier{oldX, history}
			}
		}
	}

	return nil
}
