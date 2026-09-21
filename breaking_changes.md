This file will be updated with breaking changes, before you update you should check this file for steps on updating your database schema and migration processes, and be notified of other breaking changes elsewhere.

**Privacy-affecting defaults are now opt-in**

Four configuration changes. Self-hosters who relied on the previous behaviour must now opt in
explicitly; nothing is removed, only the defaults changed.

 - `yagpdb.disable_prefix_commands` has been **renamed** to `yagpdb.enable_prefix_commands`, and
   the sense is inverted. Running built-in commands through the command prefix is now off by
   default; set `yagpdb.enable_prefix_commands=true` to restore it. Custom commands are unaffected,
   including those triggered by message content. If you previously set
   `yagpdb.disable_prefix_commands`, that key no longer does anything and must be replaced.
 - `yagpdb.enable_username_tracking` now defaults to **false**. Username and nickname history is no
   longer recorded unless you opt in, and the `usernames`, `nicknames` and `resetpastnames`
   commands stay disabled while it is off. The `pastUsernames` and `pastNicknames` custom command
   template functions return an error while disabled. Existing rows in `username_listings` and
   `nickname_listings` are left untouched; delete them yourself if you want the data gone.
 - `yagpdb.verification.track_ips` now defaults to **false**. Verification no longer records the IP
   a member verified from, which also means shared-IP alt detection and the automatic banning of
   alts of a banned user do not run unless you opt in. Existing rows keep their stored IPs.
 - `yagpdb.enable_message_log_purge` now defaults to **true**. Message logs older than 30 days are
   deleted automatically. Set it to `false` to retain logs indefinitely as before. If you are
   upgrading an instance that has never purged, the first run will delete a large backlog, so take
   a backup first if you need that history.


**18th Apr 2022 (1.32.0-dev)**

 - Autorole Full scan feature is now paid premium only, and the configuration form is disabled when full scan is active.
 - Minimum required redis version is 5.x.

**27th Jul 2019 (1.19.12-dev)**

 - You can't access old message logs unless you migrate them from the old format using the `migratelogs` owner only command. Be careful and only run this once as otherwise you'll have duplicate log entries.
 - Message log tables has been restructured, the ones used now are `messages2` and `message_logs2`. 
 - Note that this does not remove the old logs after migrating them, so you'll have to delete those tables (messages, message_logs) yourself if you want to save space.

 - If you somehow to manage to run it several times then use the following query to delete legacy imported duplicates:
```sql
 DELETE FROM message_logs2 a USING (
      SELECT MIN(ctid) as ctid, legacy_id
        FROM message_logs2 
        GROUP BY legacy_id HAVING COUNT(*) > 1
      ) b
      WHERE a.legacy_id = b.legacy_id
      AND a.ctid <> b.ctid
      AND a.legacy_id != 0;
```

**26th May 2019 (1.19-dev)**

 - Removed automigration from legacy format for reddit and custom commands, if you have them in the legacy format still you need to run the webserver on 1.18 first to migrate them to the new format, this auto-migration has been in place since 1.14, if you used any versions after that they should already be in the new format.

**4th january 2019 (1.14)**

 - Reddit feeds are now stored in postgres instead of redis, a migration will automatically start on the webserver and can be disabled by setting YAGPDB_REDDIT_DISABLE_REDIS_PQ_MIGRATION to anything but empty, automigration will be turned into opt-in instead of opt out in a couple versions, then removed.

**17th january 2019**

 - Custom commands are now stored in postgres, a migration from the old system is automtically started on the web server and can be disabled by setting YAGPDB_CC_DISABLE_REDIS_PQ_MIGRATION to anything but empty, this auto-migration will likely be removed in 2 or 3 versions (changing from opt-out to opt-in or complete removal)

**25th november 2018 (1.11.3)**

 - To use external https through a reverse proxy, e.g public facing https while yagpdb itself listens on http, set use the command line flag `-exthttps` on the webserver.

**24th november 2018 (1.10-dev)**

 - The old master slave system has now been removed in favor of a proper sharding orchestrator (github.com/jonas747/dshardorchestrator) that allows scaling shards across processes and in the future will do so over multiple machines (the latter is not fully implemented yet)

**7th november 2018 (1.10-dev)**
 - scheduled events cleanup, serverstats processing, soundboard transcoding and safebrowsing now needs a process running with the `-backgroundworkers` flag
     + Can still run it on the same process as the bot or webserver or whatever...
     + This is to support having multiple bot processes in the near future

**24th october 2018 (1.9.2-dev)**
 - `mqueue` no longer supports the postgres queue, meaning if you're upgrading from a version earlier than v1.4.7 and there's still messages in the queue then those wont be processed. Versions after v1.4.7 queued new messages to the new queue but still continued to also poll the postgres queue, so to get around this you can run v1.9.1 until it's empty then upgrade to v1.9.2 or later.
     + Things that uses mqueue: reddit, youtube, and reminders when triggered
     + To find out if theres still messages in the queue run `select * from mqueue where processed=false;` on the yagpdb db

**3rd aug 2018 (1.4-dev)**
 - dutil now only has one maintained branch, the master which was merged with dgofork.
 - my discordgo fork's default branch is now `yagpdb` instead of `master`
 - Updated build scripts (docker and circle) as a result, if your docker script isnt working in the future this is most likely the reason if you have a old version of the docker build script
