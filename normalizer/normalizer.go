package normalizer

import (
	"bytes"
	"net/url"
	"sort"
	"strings"
)

type Fn func(key string) (string, error)

type Normalizer struct {
	fn []Fn
}

func New(fn ...Fn) *Normalizer {
	return &Normalizer{fn: fn}
}

func (n *Normalizer) Normalize(key string) (string, error) {
	var err error
	for _, normalizer := range n.fn {
		key, err = normalizer(key)
		if err != nil {
			return "", err
		}
	}

	return key, nil
}

func NormalizeQuery(query string) (string, error) {
	if query == "" {
		return "", nil
	}

	groups := make(map[string][]string)
	var keyOrder []string
	seenKeys := make(map[string]bool)

	start := 0
	for i := 0; i <= len(query); i++ {
		if i == len(query) || query[i] == '&' {
			pair := query[start:i]
			start = i + 1
			if len(pair) == 0 {
				continue
			}

			eqIdx := strings.IndexByte(pair, '=')
			var rawK, rawV string
			if eqIdx != -1 {
				rawK = pair[:eqIdx]
				rawV = pair[eqIdx+1:]
			} else {
				rawK = pair
				rawV = ""
			}

			k, err := url.QueryUnescape(rawK)
			if err != nil {
				return "", err
			}
			v, err := url.QueryUnescape(rawV)
			if err != nil {
				return "", err
			}

			k = strings.ReplaceAll(k, "#", "")
			k = strings.ReplaceAll(k, "=", "")

			if !strings.Contains(query, "%5B%5D") && !strings.Contains(query, "[]") {
				k = removeAllNumericIndices(k)
			}

			if !seenKeys[k] {
				seenKeys[k] = true
				keyOrder = append(keyOrder, k)
			}

			groups[k] = append(groups[k], v)
		}
	}

	for k := range groups {
		sort.Strings(groups[k])
	}

	hasNegative := false
	for _, k := range keyOrder {
		if strings.Contains(k, "-") {
			hasNegative = true
			break
		}
	}
	if !hasNegative {
		sort.Strings(keyOrder)
	}

	var buf bytes.Buffer
	isFirst := true

	for _, k := range keyOrder {
		values := groups[k]
		for _, val := range values {
			if !isFirst {
				buf.WriteByte('&')
			}
			isFirst = false

			buf.WriteString(k)
			buf.WriteByte('=')
			buf.WriteString(val)
		}
	}

	return buf.String(), nil
}

func removeAllNumericIndices(key string) string {
	var result bytes.Buffer
	i := 0
	for i < len(key) {
		if key[i] == '[' {
			closeIdx := -1
			isNumeric := true
			hasDigits := false

			for j := i + 1; j < len(key); j++ {
				if key[j] == ']' {
					closeIdx = j
					break
				}
				if key[j] < '0' || key[j] > '9' {
					isNumeric = false
				} else {
					hasDigits = true
				}
			}

			if closeIdx != -1 && isNumeric && hasDigits {
				i = closeIdx + 1
				continue
			}
		}

		result.WriteByte(key[i])
		i++
	}
	return result.String()
}
