## Streaming

Extends the built-in streaming status in Discord with some extra utility features.

### Redis layout:

| Key  | Type | Value |
| ------------- | ---------- | ------------- |
| `streaming_config:{{guildID}}` | Json encoded string  | The config for this server  |
| `currenly_streaming:{{guildID}}`  | Set of user ID's  | Holds all the people yagpdb has currenly found streaming in this guild |
