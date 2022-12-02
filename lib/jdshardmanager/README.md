# dshardmanager

Simple shard manager for discord bots

Status: 

 - [x] Core funcitonality, add handlers, log shard events to discord and to custom outputs
 - [x] Fancy status message in discord that gets updated live 
 - [x] Use the recommded shard count by discord
 - [ ] Warn when getting close to the cap
 - [ ] Automatically re-scale the sharding when needed 
        Needed being when a shard with +2500 guilds disconnects and fails to resume, this shard will no longer be able to identify afaik
 - [ ] Simple api? maybe in an extras package.