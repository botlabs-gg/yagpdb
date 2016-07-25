# Reddit plugin for YAGPDB

Redis layout:

**global_subreddit_watch:sub**
Type: hash
Key: {{guild}}:{{watchid}}
Value: SubredditWatchItem

**guild_subreddit_watch:guildid**
Type: hash
Key: {{watchid}}
Value: SubredditWatchItem

watchid is a per guild increasing id for items
