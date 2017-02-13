# youtube feeds

###Storage layout:

Postgres tables: 
youtube_guild_subs - postgres
    - guild_id
    - channel_id
    - youtube_channel

youtube_playlist_ids
    - channel_name PRIMARY
    - playlist_id

Redis keys: 

`youtube_subbed_channels` - sorted set

key is the channel name
score is unix time in seconds when it was last checked

at the start of a poll, it uses zrange/zrevrange to grab n amount of entries to process and if they do get processed it updates the score to the current unic time


`youtube_last_video_time:{channel}` - string

holds the time of the last video in that channel we processed, all videos before this will be ignored.

`youtube_last_video_id:{channel}` - string

holds the last video id for a channel, it will stop processing videos when it hits this video