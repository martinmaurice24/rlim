local key = KEYS[1]
local max_tokens = tonumber(ARGV[1])
local consume_rate = tonumber(ARGV[2])
local reset_at = tonumber(ARGV[3])
local now = tonumber(ARGV[4])

local last_consume = redis.call('HGET', key, 'last_consume')
local tokens = redis.call('HGET', key, 'tokens')

if last_consume == false then
    last_consume = now
    tokens = 0
else
    last_consume = tonumber(last_consume)
    tokens = tonumber(tokens)

    local time_elapsed = now - last_consume
    local tokens_to_consume = time_elapsed * consume_rate
    tokens = math.max(0, tokens - tokens_to_consume)
    last_consume = now
end

if tokens <= max_tokens then
    tokens = tokens + 1
    redis.call('HSET', key, 'tokens', tokens, 'last_consume', last_consume)
    redis.call('EXPIRE', key, reset_at)
    return {1, tokens}
else
    return {0, tokens}
end

