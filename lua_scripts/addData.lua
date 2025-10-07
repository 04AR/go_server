local hash   = tostring(KEYS[1])
local action = tostring(ARGV[1])
local field  = tostring(ARGV[2])

redis.call("PUBLISH", "room1", cjson.encode({action=action, field=field, hash=hash}))

if action == "add" then
    local value = tostring(ARGV[3] or "")
    redis.call("HSET", hash, field, value)
    -- Return a JSON string
    return cjson.encode({status="ok", action="add", field=field, value=value})

elseif action == "remove" then
    redis.call("HDEL", hash, field)
    return cjson.encode({status="ok", action="remove", field=field})

else
    return cjson.encode({status="error", message="invalid action: "..action})
end

