local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local leak_rate = tonumber(ARGV[2])
local expires_at = tonumber(ARGV[3])
local now_unix = tonumber(ARGV[4])

local last_leak_unix = redis.call('HGET', key, 'last_leak_unix')
local bucket_size = redis.call('HGET', key, 'bucket_size')

if last_leak_unix == false then
    last_leak_unix = now_unix
    bucket_size = 0
else
    last_leak_unix = tonumber(last_leak_unix)
    bucket_size = tonumber(bucket_size)

    local time_elapsed = now_unix - last_leak_unix
    local tokens_to_leak = time_elapsed * leak_rate
    bucket_size = math.max(0, bucket_size - tokens_to_leak)
    last_leak_unix = now_unix
end

local current_bucket_size_plus_request_cost = bucket_size + 1
if current_bucket_size_plus_request_cost <= capacity then
    redis.call('HSET', key, 'bucket_size', current_bucket_size_plus_request_cost, 'last_leak_unix', last_leak_unix)
    redis.call('EXPIRE', key, expires_at)
    return {1, bucket_size}
else
    return {0, bucket_size}
end

