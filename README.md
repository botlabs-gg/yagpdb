YAGPDB  [![Circle CI](https://circleci.com/gh/jonas747/yagpdb.svg?style=svg)](https://circleci.com/gh/jonas747/yagpdb) 
================

### Yet another general purpose discord bot

YAGPDB is a multifunctional modular discord bot, it's modular in that plugins exist for the most part on their own (with exceptions to some lazy things in the main stylesheet), some plugins do whoever depend on other plugins, (most plugins depends on the commands plugin for example)

**Links**
 - [YAGPDB.xyz](http://yagpdb.xyz)
 - [For updates and support join my discord server](https://discord.gg/Cj6kCba)

### Running YAGPDB yourself

Running this bot may seem challenging and that's because i don't have time to make it easy to run for everyone, for the most part it should run fine after the initial work has been done.

There's also some struggle when you update, as in the past at least I've been bad at announcing what needs to be done to migrate certain data, in most cases `-a "migrate"` will do the trick though.

With that said running this bot requires knowledge of:

 - Basic batch/shell scripting (as in setting environment variables and such, really basic stuff)
 - Basic knowledge of postgresql (being able to create the database and user for yagpdb)
 - Basic knowledge of redis (being able to install and run it)
 - Basic knowledge of go (being able to compile things)
 - Basic knowledge of git (being able to change branches and such)

**I will not help you if you're missing one of these, i simply do not have time.**

(There's still a lot of contributing you can do without this though, such as writing docs, fixing my horribel typos and so on)

**Running YAGPDB on your own however you can expect little to no support on helping it get set up, unless the purpose of you setting it up is to help out the project.**

Required databases: 
 - PostgresSQL
     + requires a db named yagpdb, this is currently hardcoded in.
     + The user and password is configurable through env variables.
 - Redis
     + Defaults are fine

First step is to set those up, and get them running.

**The webserver currently requires a domain, (a subdomain works), reason is currently i haven't made https (letsencrypt) optional.**

The webserver by default (unless `-pa`) listens on 5000(http) and 5001(https)
So if you're behind a NAT, forward those, if not you can either use the `-pa` switch or add an entry to iptables.

**Steps for building:**

YAGPDB currently use a lot of alternative branches of my projects, mostly because of my custom discordgo fork.

1. `go get github.com/jonas747/yagpdb/cmd/yagpdb` (this will error, that's fine)
2. go into `$GOPATH/src/github.com/jonas747/discordgo` and change the git branch to `yagpdb`
2. go into `$GOPATH/src/github.com/jonas747/dutil` and change the git branch to `dgofork`
3. cd `${GOPATH}/src/github.com/jonas747/dshardmanager && git checkout dgofork`
4. cd `${GOPATH}/src/github.com/jonas747/dcmd && git checkout dgofork`

You can now build it and run `$GOPATH/src/github.com/jonas747/yagpdb/cmd/yagpdb`

Configuration is done through environment variables. See `cmd/yagpdb/sampleenvfile` for which environment variables there are.

You can run the webserver, bot, reddit and youtube parts as seperate processes (haven't tested on different physical machines yet, doubt it'll work well atm for the webserver and bot at least)

You specify `-bot` to run the bot, `-web` to run the webserver and so on.
And it should be running now.

### Plugins

**Standard plugins:**

* Youtube-Feed
* Stream-announcements
* Soundboard
* Serverstats
* Reputation
* Reminder
* Reddit-Feed
* Notifications
* Moderation
* Logs
* Customcommands
* And More!

**Planned plugins**

[See the Issues-Tab for more](https://github.com/jonas747/yagpdb/issues)

### Core packages:

- Web: The core webserver package, in charge of authentication.
- Bot: Core bot package, delegates events to plugins.
- Common: Handles all the common stuff between web and bot (config, discord session, redis pool etc).
- Feeds: Handles all feeds, personally I like to run each feed as its own service, that way I can start and stop individual feed without   taking everything down.
- Commands: Handles all commands.

Currently YAGPDB builds everything into one executable and you run the bot with the -bot switch, the webserver with the -web switch and so on (see -h for help).

### Databases

YAGPDB uses redis for light data and caching. (and some remnants of configuration)

It uses postgresql for most configuration and more heavy data (logs and such)

### Contributing new features

See bot/plugin for info about bot plugins, web/plugin for web plugins and feeds/plugin for feeds, if you wanna make a new fully fledged plugin.

Expect web, bot and feed instances to be run separately.

For basic utility/fun commands you can just jam them in stdcommands, use the existing commands there as an example of how to add one.

**If you need any help finding things in the source or have any other questions, don't be afraid of messaging me**
