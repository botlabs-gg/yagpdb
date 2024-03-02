Package quackmium provides functionality to give guilds quackmium status through various sources (patreon only atm).

It maintains a list of quackmium users and guilds and their slots in redis, compiled from each source.

Those redis lists/sets/hashes are updated at a certain interval from the sources, that means no matter how many sources you only have to check 1 key to see if a guild is quackmium, it also simplifies things as a whole.
