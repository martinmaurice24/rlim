local key = KEYS[1]
local max_tokens = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local reset_at = tonumber(ARGV[3])
local now = tonumber(ARGV[4])

local last_refill = redis.call('HGET', key, 'last_refill')
local tokens = redis.call('HGET', key, 'tokens')

if last_refill == false then
    last_refill = now
    tokens = max_tokens
else
    last_refill = tonumber(last_refill)
    tokens = tonumber(tokens)

    local time_elapsed = now - last_refill
    local tokens_to_refill = time_elapsed * refill_rate
    tokens = math.min(max_tokens, tokens + tokens_to_refill)
    last_refill = now
end

if tokens > 0 then
    tokens = tokens - 1
    redis.call('HSET', key, 'tokens', tokens, 'last_refill', last_refill)
    redis.call('EXPIRE', key, reset_at)
    return {1, tokens}
else
    return {0, tokens}
end

