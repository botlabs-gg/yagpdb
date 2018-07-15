Yagmaster manages the zero downtime restarts of the bot process.

It also launches the bot processes itself and manages them, it starts them with `-bot` and `-syslog` hardcoded at the moment, this will probably be epxosed to configuration later.

To signal migration to a new process send yagmaster `SIGUSR1`, this means it currently only works on linux.

It currently does not support shard rescaling, that requires a cold restart still.
