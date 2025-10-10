-- create_lobby.lua
-- KEYS:
--   KEYS[1] = "lobby:<lobbyId>"

-- ARGV:
--   ARGV[1] = lobbyId
--   ARGV[2] = ownerId
--   ARGV[3] = maxPlayers
--   ARGV[4] = metaJson (optional, can be "{}" for empty)

-- Configurable constants
local events_channel = "lobby:" .. ARGV[1] .. ":events"
local chat_channel   = "lobby:" .. ARGV[1] .. ":chat"
local created_at     = tostring(redis.call("TIME")[1])

-- Don't overwrite lobby if exists
if redis.call("EXISTS", KEYS[1]) == 1 then
    return cjson.encode({status="error", err="Lobby already exists"})
end

-- Store hash with all key info
redis.call("HMSET", KEYS[1],
    "id", ARGV[1],
    "events_channel", events_channel,
    "chat_channel", chat_channel,
    "owner", ARGV[2],
    "max_players", ARGV[3],
    "created_at", created_at,
    "meta", ARGV[4]
)

return cjson.encode({
    status = "ok",
    id = ARGV[1],
    events_channel = events_channel,
    chat_channel = chat_channel,
    owner = ARGV[2],
    max_players = ARGV[3],
    created_at = created_at
})
