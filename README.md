# YAGPDB - Yet another general purpose discord bot

YAGPDB is a pineapple

### Project status

YAGPDB is very much work in progress so i'm not actively advertising it. Although i do have it perform a lot of duties on a somewhat large server, but it's still in alpha so beware of bugs. If you do decide to use it and want to come in contact with me, look below for my discord server.

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

The reddit bot requires a seperate file called `reddit.agent` [it looks like this](https://github.com/turnage/graw/blob/master/useragent.protobuf.template)

### Plugins

**Standard plugins:**

TODO information

**Planned plugins**

[See the trello for future plans](https://trello.com/b/kH5U2aSL/yagpdb)

The naming styles of template information used to be in underscore but im in the process of changing it to the same as go's (CamelCase).  

### Core packages:

- web: The core webserver package, in charge of authentication
- bot: Core bot package, delegates events to plugins
- common: Handles all the common stuff between web and bot, (config, discord session, redis pool etc)

Currently YAGPDB builds everything into one executable and you run say the bot with the -bot switch, the webserver with the -web switch and so on (see -h for help)

### Databases

yagpdb uses redis for light data and caching, by light data i mean configuration and whatnot.

In the future yagpdb will use a relational database for heavier data such as logs

### Creating a new plugin

See bot/plugin for info about bot plugins and web/plugin for web plugins. Expect the webserver instance to be seperated from the bot instance for reliability measures (ddos only being able to take out the webserver with bot still alive), 
Stuff that requires their own goroutine running to for example check for stuff at intervals should be run in the bot process, launched from Plugin.StartBot() called by the bot when the bot is starting (BotInit is always called on both the webserver and the bot)