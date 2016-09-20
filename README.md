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

For configuration create a config.json file in the same dir as the built binary, for what you should include see [config.go](https://github.com/jonas747/yagpdb/blob/master/common/config.go#L8) 

The reddit bot requires a seperate file called `reddit.agent` [it looks like this](https://github.com/turnage/graw/blob/master/useragent.protobuf.template)

### Plugins

**Standard plugins:**

 - notifications
     + Provides general notifications about dsicord events
     + Events:
         * member joins, leave, topic change
 - commands
     + This plugin provides utilities for configuring global command settings
     + It also provides some fun utility commands
 - customcommands
     + Custom commands is a plugin which lets server admins define their own custom commands
     + Currently only simple custom commands can be made which responds with a fixed message on a trigger, more is planned 
 - aylien
     + A fun plugin that provides access to the aylien text analysys api
 - serverstats
     + Tracks stats on your server and you can optionally make them public
 - reddit
     + Posts post from a subreddit into a discord channel
     + Optimally it should be posted within 1 minute of the post being posted but should that fail it might take some time for the secondary crawler to find it
 - reputation
     + Allows people to give reputation (rep) to eachother

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

