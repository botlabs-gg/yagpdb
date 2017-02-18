YAGPDB  [![Circle CI](https://circleci.com/gh/jonas747/yagpdb.svg?style=svg)](https://circleci.com/gh/jonas747/yagpdb) 
================
    

YAGPDB is a pineapple

### Project status

YAGPDB is very much work in progress so I'm not actively advertising it. Although i do have it perform a lot of duties on a somewhat large server, but it's still in alpha so beware of bugs. If you do decide to use it and want to come in contact with me, look below for my discord server.

**Links**
 - [YAGPDB.xyz](http://yagpdb.xyz)
 - [For updates and support join my discord server](https://discord.gg/Cj6kCba)

### Running YAGPDB yourself

Currently i do not provide precompiled binaries so you will have to compile it yourself, if you do not understnad the following steps then i advice you not to run it.

1. `go get github.com/jonas747/yagpdb/cmd/yagpdb`
2. go into `$GOPATH/src/github.com/jonas747/discordgo` and change the git branch to `yagpdb`
2. go into `$GOPATH/src/github.com/jonas747/dutil` and change the git branch to `dgofork`

You can now build it and run `$GOPATH/src/github.com/jonas747/yagpdb/cmd/yagpdb`

Configuration is done through environment variables. see `cmd/yagpdb/sampleenvfile` for which environment variables there are


### Plugins

**Standard plugins:**

TODO information

**Planned plugins**

[See the trello for future plans](https://trello.com/b/kH5U2aSL/yagpdb)

### Core packages:

- web: The core webserver package, in charge of authentication
- bot: Core bot package, delegates events to plugins
- common: Handles all the common stuff between web and bot, (config, discord session, redis pool etc)
- feeds: Handles all feeds, personally i like to run each feed as it's own service, that way i can start and stop individual feed without taking everything down.
- commands: Handles all commands, currently in the process of making a new command system based on interfaces instead.

Currently YAGPDB builds everything into one executable and you run say the bot with the -bot switch, the webserver with the -web switch and so on (see -h for help)

### Databases

yagpdb uses redis for light data and caching.

I'm currently in the process of moving configuration over to postgres, about 1/3 of the configuration settings lives on postgres atm.

### Creating a new plugin

See bot/plugin for info about bot plugins, web/plugin for web plugins and feeds/plugin for feeds.

Expect web, bot and feed instances to be run separately and also expect there to be multiple web and bot instances. for reliability and scalability measures (splitting load across multiple machines in the future and such)