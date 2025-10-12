-- Lua-generated lobby id create script
-- KEYS:
--   KEYS[1] = "lobby:<lobbyId>"

-- ARGV:
--   ARGV[1] = maxPlayers (optional)

if redis.call("EXISTS", KEYS[1]) == 1 then
    return cjson.encode({status="error", err="Lobby already exists"})
end

-- Parse maxPlayers (default 10)
local maxPlayers = tonumber(ARGV[1])
if maxPlayers == nil or maxPlayers < 1 then
    maxPlayers = 10
end

local created_at = tostring(redis.call("TIME")[1])

-- Create the lobby hash
redis.call("HMSET", KEYS[1],
    "id", KEYS[1]:sub(7),  -- Extract lobbyId from key
    "max_players", maxPlayers,
    "created_at", created_at
)

return cjson.encode({
    status = "ok",
    id = KEYS[1],
    max_players = maxPlayers,
    created_at = created_at
})
