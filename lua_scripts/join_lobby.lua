-- KEYS:
--   KEYS[1] = "lobby:<lobbyId>"
--   KEYS[2] = "lobby:<lobbyId>:players"

-- ARGV:
--   ARGV[1] = lobbyId
--   ARGV[2] = playerId
--   ARGV[3] = playerStateJson (e.g. {"health":100,"x":0,"y":0})

-- Step 1: Check lobby exists
if redis.call("EXISTS", KEYS[1]) == 0 then
    return cjson.encode({status="error", err="Lobby does not exist"})
end

-- Step 2: Check if already joined
if redis.call("HEXISTS", KEYS[2], ARGV[2]) == 1 then
    return cjson.encode({status="error", err="Already in lobby"})
end

-- Step 3: Check lobby full
local maxPlayers = tonumber(redis.call("HGET", KEYS[1], "max_players"))
local numPlayers = tonumber(redis.call("HLEN", KEYS[2]))
if numPlayers >= maxPlayers then
    return cjson.encode({status="error", err="Lobby full"})
end

-- Step 4: Add player state
redis.call("HSET", KEYS[2], ARGV[2], ARGV[3])

-- Step 5: Publish join event
local events_channel = redis.call("HGET", KEYS[1], "events_channel")
local evt = cjson.encode({
    type = "player_joined",
    lobby_id = ARGV[1],
    player_id = ARGV[2],
    state = ARGV[3]
})
redis.call("PUBLISH", events_channel, evt)

-- Step 6: Return success
return cjson.encode({
    status = "ok",
    player_id = ARGV[2],
    lobby_id = ARGV[1],
    current_players = numPlayers + 1
})
