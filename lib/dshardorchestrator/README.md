dshardorchestrator provides a sharding orchestrator for discord bots.

It's purpose is to manage nodes and assign shards to them, aswell as migrating shards between the nodes, which is one method of scaling large discord bots, spreading shards across processes and servers.

A here is a "process", which can be spread over hosts if you want.

I would not recommend using this, as 1. it currently only works against my discordgo fork and 2. its lacking a lot of tests still (although this im improving on)

It's currently used in YAGPDB.



# Pitfalls

Currently its somewhat easy to break, if you try to break it that is, im working towards that but yeah, in it's current state i would just not recommend using it, unless you know what you're doing.

Essentials TODO:

 - Full upgrade (simple function to migrate all nodes)
 - Add in safeguards for doing things like, stopping nodes in the middle of migration.


Later:
 - Extended status polling form nodes
 - 