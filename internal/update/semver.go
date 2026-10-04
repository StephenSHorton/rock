package update

import (
	"strconv"
	"strings"
)

// Normalize strips a leading v and surrounding space.
func Normalize(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

// Compare returns -1 if a < b, 0 if equal, 1 if a > b.
// "dev" and empty sit below every numbered release.
func Compare(a, b string) int {
	an, bn := Normalize(a), Normalize(b)
	if an == bn {
		return 0
	}
	if !numbered(an) && numbered(bn) {
		return -1
	}
	if numbered(an) && !numbered(bn) {
		return 1
	}
	as, bs := parts(an), parts(bn)
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		av, bv := 0, 0
		if i < len(as) {
			av = as[i]
		}
		if i < len(bs) {
			bv = bs[i]
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}

func numbered(v string) bool {
	if v == "" || v == "dev" || v == "devel" || v == "(devel)" {
		return false
	}
	return v[0] >= '0' && v[0] <= '9'
}

func parts(v string) []int {
	head, _, _ := strings.Cut(v, "-")
	head, _, _ = strings.Cut(head, "+")
	var out []int
	for _, p := range strings.Split(head, ".") {
		n, _ := strconv.Atoi(p)
		out = append(out, n)
	}
	return out
}
