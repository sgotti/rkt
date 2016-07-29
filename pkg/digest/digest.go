package digest

import (
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"regexp"
	"strings"
)

type Algorithm string

const (
	SHA256 Algorithm = "sha256"
	SHA512 Algorithm = "sha512"
)

var (
	algorithms = map[Algorithm]func() hash.Hash{
		SHA256: sha256.New,
		SHA512: sha512.New,
	}
)

var digestRegexp = regexp.MustCompile(`[a-zA-Z0-9-_+.]+-[a-fA-F0-9]+`)

func (a Algorithm) PrefixLen() int {
	return len(a)
}

// Digester generates a digest for a given hash algorithm
type Digester struct {
	a    Algorithm
	hash hash.Hash
}

// NewDigester returns a digester for the given hash algorithm
func NewDigester(a Algorithm) *Digester {
	return &Digester{
		a:    a,
		hash: algorithms[a](),
	}
}

// Hash returns the digester hash.Hash
func (d *Digester) Hash() hash.Hash {
	return d.hash
}

// Len return the length in characters of a full digest string
func (d *Digester) Len() int {
	return len(d.a) + len("-") + d.hash.Size()*2 // hex
}

// Digest calculates the digest
func (d *Digester) Digest() string {
	s := d.hash.Sum(nil)
	if len(s) != d.hash.Size() {
		panic(fmt.Sprintf("bad hash: %x", s))
	}
	return fmt.Sprintf("%s-%x", string(d.a), s)
}

// Perse digest parses and returns the normalized digest (replacing colon with
// hypen) and the hash algorithm
func ParseDigest(ds string) (string, Algorithm, error) {
	// Replace possible colon with hypen
	ds = strings.Replace(ds, ":", "-", 1)
	if !digestRegexp.MatchString(ds) {
		return "", "", fmt.Errorf("wrong digest")
	}
	parts := strings.Split(ds, "-")
	a := parts[0]
	if _, ok := algorithms[Algorithm(a)]; !ok {
		return "", "", fmt.Errorf("unknown digest")
	}
	return ds, Algorithm(a), nil
}
