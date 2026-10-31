package service

// CUSTOM: Version ordering mirrors qxapi-client/internal/update.
import (
	"cmp"
	"regexp"
	"strconv"
	"strings"
)

// clientVersionReleased reports whether v is a release version rather than a build from
// source ("dev", "0bcb2cc-dirty", git describe's "v0.1.0-3-g0bcb2cc");
// only releases update themselves.
func clientVersionReleased(v string) bool {
	s := parseClientVersion(v)
	return s != nil && !clientVersionDescribe.MatchString(s.pre) && !strings.Contains(s.pre, "dirty")
}

// clientVersionDescribe matches what git describe adds after a tag: commits since, hash.
var clientVersionDescribe = regexp.MustCompile(`^\d+-g[0-9a-f]+`)

// clientVersionNewer reports whether version a comes after b. A pre-release comes before
// the release it leads up to.
func clientVersionNewer(a, b string) bool {
	x, y := parseClientVersion(a), parseClientVersion(b)
	if x == nil || y == nil {
		return false
	}
	for i := range 3 {
		if x.n[i] != y.n[i] {
			return x.n[i] > y.n[i]
		}
	}
	return compareClientVersionPre(x.pre, y.pre) > 0
}

// compareClientVersionPre orders two pre-releases the way semver does (§11): no
// pre-release comes after any; otherwise identifier by identifier, numbers
// as numbers ("beta.10" after "beta.9"), a number before a word, and a
// shorter list before a longer one it starts.
func compareClientVersionPre(a, b string) int {
	switch {
	case a == b:
		return 0
	case a == "":
		return 1
	case b == "":
		return -1
	}
	x, y := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(x) && i < len(y); i++ {
		m, errM := strconv.ParseUint(x[i], 10, 64)
		n, errN := strconv.ParseUint(y[i], 10, 64)
		switch {
		case errM == nil && errN == nil:
			if m != n {
				return cmp.Compare(m, n)
			}
		case errM == nil:
			return -1
		case errN == nil:
			return 1
		default:
			if c := strings.Compare(x[i], y[i]); c != 0 {
				return c
			}
		}
	}
	return cmp.Compare(len(x), len(y))
}

type clientSemver struct {
	n   [3]int
	pre string
}

func parseClientVersion(v string) *clientSemver {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	// build metadata ("+sha.5d0c1dc") plays no part in the order
	v, _, _ = strings.Cut(v, "+")
	v, pre, _ := strings.Cut(v, "-")
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return nil
	}
	var s clientSemver
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nil
		}
		s.n[i] = n
	}
	s.pre = pre
	return &s
}
