-- create_lobby.lua
-- KEYS:
--   KEYS[1] = "lobby:<lobbyId>"

-- ARGV:
--   ARGV[1] = lobbyId
--   ARGV[2] = maxPlayers

-- Default maxPlayers to 10 if not provided or not a valid number
local maxPlayers = tonumber(ARGV[2])
if maxPlayers == nil or maxPlayers < 1 then
    maxPlayers = 10
end

-- Configurable constants
local created_at = tostring(redis.call("TIME")[1])

-- Don't overwrite lobby if exists
if redis.call("EXISTS", KEYS[1]) == 1 then
    return cjson.encode({status="error", err="Lobby already exists"})
end

-- Store hash with all key info
redis.call("HMSET", KEYS[1],
    "id", ARGV[1],
    "max_players", maxPlayers,
    "created_at", created_at
)

return cjson.encode({
    status = "ok",
    id = ARGV[1],
    -- owner = ARGV[2],
    max_players = ARGV[3],
    created_at = created_at
})
