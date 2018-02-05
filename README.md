YAGPDB  [![Circle CI](https://circleci.com/gh/jonas747/yagpdb.svg?style=svg)](https://circleci.com/gh/jonas747/yagpdb) 
================
    

YAGPDB is a pineapple!

### Project status

YAGPDB is very much work in progress so I'm not actively advertising it. Although I do have it perform a lot of duties on a somewhat large server, but it's still in alpha so beware of bugs. If you do decide to use it and want to come in contact with me, look below for my discord server.

**Links**
 - [YAGPDB.xyz](http://yagpdb.xyz)
 - [For updates and support join my discord server](https://discord.gg/Cj6kCba)

### Running YAGPDB yourself

**Running YAGPDB on your own you can expect little to no support on helping it get set up, unless the purpose of you setting it up is to help out the project.**

**IF YOU DO NOT UNDERSTAND THESE STEPS THEN DON'T BOTHER TRYING TO RUN THIS, Please dont waste my time by ignoring this warning**

Required databases: 
 - PostgresSQL
     + requires a db named yagpdb, this is currently hardcoded in.
     + The user and password is configurable through env variables.
 - Redis

First step is to get all the deps, change to proper branches and build it.

1. `go get github.com/jonas747/yagpdb/cmd/yagpdb`
2. go into `$GOPATH/src/github.com/jonas747/discordgo` and change the git branch to `yagpdb`
2. go into `$GOPATH/src/github.com/jonas747/dutil` and change the git branch to `dgofork`
3. cd `${GOPATH}/src/github.com/jonas747/dshardmanager && git checkout dgofork`


You can now build it and run `$GOPATH/src/github.com/jonas747/yagpdb/cmd/yagpdb`

Configuration is done through environment variables. See `cmd/yagpdb/sampleenvfile` for which environment variables there are.


### Plugins

**Standard plugins:**

* Youtube-Feed
* Streamannouncement
* Soundboard
* Serverstats
* Reputation
* Reminder
* Reddit-Feed
* Notifications
* Moderation
* Logs
* Customcommands

**Planned plugins**

[See the Issues-Tab for more](https://github.com/jonas747/yagpdb/issues)

### Core packages:

- Web: The core webserver package, in charge of authentication.
- Bot: Core bot package, delegates events to plugins.
- Common: Handles all the common stuff between web and bot (config, discord session, redis pool etc).
- Feeds: Handles all feeds, personally I like to run each feed as its own service, that way I can start and stop individual feed without   taking everything down.
- Commands: Handles all commands, currently in the process of making a new command system based on interfaces instead.

Currently YAGPDB builds everything into one executable and you run the bot with the -bot switch, the webserver with the -web switch and so on (see -h for help).

### Databases

YAGPDB uses redis for light data and caching.

I'm currently in the process of moving configuration over to postgres, about 50% of the configuration settings lives on postgres atm.

### Creating a new plugin

See bot/plugin for info about bot plugins, web/plugin for web plugins and feeds/plugin for feeds.

Expect web, bot and feed instances to be run separately and also expect there to be multiple web and bot instances for reliability and scalability measures (splitting load across multiple machines in the future and such).
