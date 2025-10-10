-- KEYS:
--   KEYS[1] = "lobby:<lobbyId>"
--   KEYS[2] = "lobby:<lobbyId>:players"

-- ARGV:
--   ARGV[1] = lobbyId
--   ARGV[2] = playerId
--   ARGV[3] = playerStateJson (e.g. {"health":80,"x":5,"y":6})

-- Step 1: Check lobby exists
if redis.call("EXISTS", KEYS[1]) == 0 then
    return cjson.encode({status="error", err="Lobby does not exist"})
end

-- Step 2: Check player is in the lobby
if redis.call("HEXISTS", KEYS[2], ARGV[2]) == 0 then
    return cjson.encode({status="error", err="Player not in lobby"})
end

-- Step 3: Update player state
redis.call("HSET", KEYS[2], ARGV[2], ARGV[3])

-- Step 4: Publish to state channel
local state_channel = "lobby:" .. ARGV[1] .. ":events"
local evt = cjson.encode({
    type = "player_state_update",
    lobby_id = ARGV[1],
    player_id = ARGV[2],
    state = ARGV[3]
})
redis.call("PUBLISH", state_channel, evt)

-- Step 5: Success response
return cjson.encode({
    status = "ok",
    lobby_id = ARGV[1],
    player_id = ARGV[2]
})
