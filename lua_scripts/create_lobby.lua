-- create_lobby.lua
-- Arguments:
--   ARGV[1] = lobby name (string, required)
--   ARGV[2] = userId (string, required)

-- Configurable constants
local MAX_LOBBY_NAME_LEN = 32
local MIN_LOBBY_NAME_LEN = 3
local MAX_MEMBERS = 50

-- Utility: return structured response
local function ok(data)
    return cjson.encode({ status = "ok", data = data })
end

local function err(code, msg)
    return cjson.encode({ status = "error", error = code, message = msg })
end

-- Validate args
if not ARGV[1] or ARGV[1] == "" then
    return err("invalid_name", "Lobby name is required")
end
if not ARGV[2] or ARGV[2] == "" then
    return err("invalid_user", "User ID is required")
end

local lobbyName = ARGV[1]
local userId    = ARGV[2]

if string.len(lobbyName) < MIN_LOBBY_NAME_LEN then
    return err("name_too_short", "Lobby name must be at least " .. MIN_LOBBY_NAME_LEN .. " characters")
end
if string.len(lobbyName) > MAX_LOBBY_NAME_LEN then
    return err("name_too_long", "Lobby name must be at most " .. MAX_LOBBY_NAME_LEN .. " characters")
end

local lobbyKey = "lobby:" .. lobbyName
local membersKey = lobbyKey .. ":members"
local ownerKey = lobbyKey .. ":owner"

-- Check if lobby already exists
if redis.call("EXISTS", lobbyKey) == 1 then
    return err("lobby_exists", "Lobby '" .. lobbyName .. "' already exists")
end

-- Create lobby hash (metadata)
redis.call("HSET", lobbyKey,
    "name", lobbyName,
    "created_at", tostring(redis.call("TIME")[1]),
    "owner", userId,
    "max_members", MAX_MEMBERS
)

-- Add owner as first member
redis.call("SADD", membersKey, userId)

-- Set owner separately for quick lookup
redis.call("SET", ownerKey, userId)

-- Optionally, set TTL if you want lobbies to expire after inactivity
-- redis.call("EXPIRE", lobbyKey, 3600)

return ok({
    lobby = lobbyName,
    owner = userId,
    members = 1,
    max_members = MAX_MEMBERS
})
