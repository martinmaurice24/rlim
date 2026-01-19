package rate_limiter

type Rate struct {
	Key                        string  `json:"key"`
	Capacity                   string  `json:"capacity"`
	RefillRate                 float64 `json:"refill_rate"`
	Tier                       string  `json:"tier"`
	ExpirationTimeoutInSeconds int     `json:"expiration_timeout_in_seconds"`
	RemainingTokens            int     `json:"tokens" redis:"tokens"`
	LastRefill                 int     `json:"last_refill" redis:"last_refill"`
}
