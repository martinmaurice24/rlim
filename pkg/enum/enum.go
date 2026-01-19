package enum

type Algorithm int

const (
	TokenBucket Algorithm = iota
	LeakyBucket
)

func (t Algorithm) String() string {
	return [...]string{"token_bucket", "leaky_bucket"}[t]
}
