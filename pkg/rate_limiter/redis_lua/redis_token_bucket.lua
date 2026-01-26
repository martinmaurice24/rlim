local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local expires_at = tonumber(ARGV[3])
local now_unix = tonumber(ARGV[4])

local last_refill_unix = redis.call('HGET', key, 'last_refill_unix')
local bucket_size = redis.call('HGET', key, 'bucket_size')

if last_refill_unix == false then
    last_refill_unix = now_unix
    bucket_size = capacity
else
    last_refill_unix = tonumber(last_refill_unix)
    bucket_size = tonumber(bucket_size)

    local time_elapsed = now_unix - last_refill_unix
    local tokens_to_refill = time_elapsed * refill_rate
    bucket_size = math.min(capacity, bucket_size + tokens_to_refill)
    last_refill_unix = now_unix
end

if bucket_size >= 1 then
    redis.call('HSET', key, 'bucket_size', bucket_size - 1, 'last_refill_unix', last_refill_unix)
    redis.call('EXPIRE', key, expires_at)
    return {1, bucket_size}
else
    return {0, bucket_size}
end

