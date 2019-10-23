# Reddit plugin for YAGPDB

### How the feed works

Uses the same method MEE6 uses for reliable cost effective scanning of all reddit posts.

Works by by manually checking 100 ids per couple of seconds via the `/api/info.json?id=t3_id1,t3_id2` etc route.

It then updates the cursor to the highest returned post id, continuing from that next call.

I don't believe this method has any faults at this moment, it seems to be more than enough even at a 5 second interval, unlike the polling of /all/new this does not appear to have any limitations on old posts and also shows absolutely all posts.

### Redis layout:

`reddit_last_post_id` id of last post processed

**global_subreddit_watch:sub**
Type: hash
Key: `{{guild}}:{{watchid}}`
Value: SubredditWatchItem

**guild_subreddit_watch:guildid**
Type: hash
Key: `{{watchid}}`
Value: SubredditWatchItem

`watchid` is a per guild increasing id for items
