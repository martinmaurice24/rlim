local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local leak_rate = tonumber(ARGV[2])
local reset_at = tonumber(ARGV[3])
local now = tonumber(ARGV[4])

local last_leak = redis.call('HGET', key, 'last_leak')
local tokens = redis.call('HGET', key, 'tokens')

if last_leak == false then
    last_leak = now
    tokens = 0
else
    last_leak = tonumber(last_leak)
    tokens = tonumber(tokens)

    local time_elapsed = now - last_leak
    local tokens_to_consume = time_elapsed * leak_rate
    tokens = math.max(0, tokens - tokens_to_consume)
    last_leak = now
end

if tokens <= capacity then
    tokens = tokens + 1
    redis.call('HSET', key, 'tokens', tokens, 'last_leak', last_leak)
    redis.call('EXPIRE', key, reset_at)
    return {1, tokens}
else
    return {0, tokens}
end

