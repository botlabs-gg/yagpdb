# Server stats plugin for YAGPDB

Records and shows stats for individual servers.

### Current stats

**Simple temporary stats**:

 - Messages last hour
 - Users joined/left today
 - Messages today, per channel with bar graphs
 - Current online users
 - Total amount of users

### Planned soon

**More peristent graphable stats**:

 - Users joined/left day to day chart
 - Avg. users online day to day
 - Messages day to day per channel


### Redis layout

To count 24h stats yagpdb stores things inside sorted sets with unix timestamp as score, it will then routinely walk over all stats and remove those with screos of less then current unix time - 24h, this might be expensive later on, if so i might have to figure out a better way.

guild_stats_msg_channel:{guildid} - sorted set: key: channelid:msg_id, score: unix timestamp
guild_stats_members_changed:{guildid} - sorted set: key: joined|left:userid, score: unix timestamp
