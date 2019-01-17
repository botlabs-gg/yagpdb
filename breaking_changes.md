This file will be updated with breaking changes, before you update you should check this file for steps on updating your database schema and migration processes, and be notified of other breaking changes elsewhere.

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
 - mqueue no longer supports the postgres queue, meaning if you're upgrading from a version earlier than v1.4.7 and there's still messages in the queue then those wont be processed. Versions after v1.4.7 queued new messages to the new queue but still continued to also poll the postgres queue, so to get around this you can run v1.9.1 until it's empty then upgrade to v1.9.2 or later.
     + Things that uses mqueue: reddit, youtube, and reminders when triggered
     + To find out if theres still messages in the queue run `select * from mqueue where processed=false;` on the yagpdb db

**3rd aug 2018 (1.4-dev)**
 - dutil now only has one maintained branch, the master which was merged with dgofork.
 - my discordgo fork's default branch is now yagpdb
 - Updated build scripts (docker and circle) as a result, if your docker script isnt working in the future this is most likely the reason if you have a old version of the docker build script