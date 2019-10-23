# YouTube Feeds

The feed plugin is complicated because we use the user's `uploads` playlist, and that is sorted by actual upload date instead of publish date.

So say you have this channel's videos:

 0. vid2 - uploaded 10pm - published 10pm
 1. vid1 - uploaded 6pm - published 6pm

Then a video was published (but uploaded a long time ago):

 0. vid2 - uploaded 10pm - published 10pm
 1. vid1 - uploaded 6pm - published 6pm
 2. vid3 - uploaded 5pm - published 11pm

vid3 was published after the latest video, but still appears at the bottom. This causes issues, as we now have no idea when to stop looking. Currently YAGPDB handles this fine as long as it's not uploaded longer than 50 videos ago, in which case it may or may not catch it.

In the future, I'll do a hybrid mode with search. Those super late published videos however will show up in Discord super late. I cannot use search for 100% either because it costs 100 times for api quota to use, meaning it could be up to hours behind.

### Storage layout:

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

`youtube_registered_websub_channels` - sorted set

key is the channel name
score is unix time in seconds when it expires

At the start of a poll, it uses zrange/zrevrange to grab an amount of entries to process, and if they do get processed, it updates the score to the current unix time.


`youtube_last_video_time:{channel}` - string

Holds the time of the last video in the processed channel. All videos before this will be ignored.

`youtube_last_video_id:{channel}` - string

Holds the last video id for a channel, it will stop processing videos when it hits this video.

`youtube_push_registrations` - sorted set

Key is the channel id, value is the time it expires

`youtube_currently_adding:{channelid}` - set, set when this channel is being added
