This file will be updated with breaking changes, before you update you should check this file for steps on updating your database schema and migration processes, and be notified of other breaking changes elsewhere.

**24th october 2018 (1.9.2-dev)**
 - mqueue no longer supports the postgres queue, meaning if you're upgrading from a version earlier than v1.4.7 and there's still messages in the queue then those wont be processed. Versions after v1.4.7 queued new messages to the new queue but still continued to also poll the postgres queue, so to get around this you can run v1.9.1 until it's empty then upgrade to v1.9.2 or later.

**3rd aug 2018 (1.4-dev)**
 - dutil now only has one maintained branch, the master which was merged with dgofork.
 - my discordgo fork's default branch is now yagpdb
 - Updated build scripts (docker and circle) as a result, if your docker script isnt working in the future this is most likely the reason if you have a old version of the docker build script