YAGPDB  [![Circle CI](https://circleci.com/gh/jonas747/yagpdb.svg?style=svg)](https://circleci.com/gh/jonas747/yagpdb) 
================

### Yet another general purpose discord bot

YAGPDB is a multifunctional modular discord bot, it's modular in that plugins exist for the most part on their own (with exceptions to some lazy things in the main stylesheet), some plugins do however depend on other plugins, (most plugins depends on the commands plugin for example)

**Links**
 - [YAGPDB.xyz](http://yagpdb.xyz)
 - [For updates and support join my discord server](https://discord.gg/Cj6kCba)

### Running YAGPDB yourself

Running this bot may seem challenging and that's because I don't have time to make it easy to run for everyone, for the most part it should run fine after the initial work has been done.

There's also some struggle when you update, as in the past at least I've been bad at announcing what needs to be done to migrate certain data, in most cases `-a "migrate"` will do the trick though.

With that said running this bot requires knowledge of:

 - Basic batch/shell scripting (as in setting environment variables and such, really basic stuff)
 - Basic knowledge of postgresql (being able to create the database and user for yagpdb)
 - Basic knowledge of redis (being able to install and run it)
 - Basic knowledge of go (being able to compile things)
 - Basic knowledge of git (being able to change branches and such)

**I will not help you if you're missing one of these, I simply do not have time. You can expect little to no support on helping it get set up, unless the purpose of you setting it up is to help out the project.**

(There's still a lot of contributing you can do without this though, such as writing docs, fixing my horrible typos and so on)

**The web server requires a domain.**

The web server (control panel) requires a domain (e.g., yagpdb.example.com) in
order to integrate with Discord. Although it is possible to run this bot without
the control panel, it is significantly more difficult and not supported by the
maintainers.

#### General Discord bot setup

Directions on creating an app and getting credentials may be found
[here](https://github.com/reactiflux/discord-irc/wiki/Creating-a-discord-bot-&-getting-a-token).
YAGPDB does not require you to authorize the bot: all of that will be handled
via the Control Panel.

In addition, you will need to add your domain to the bot's "REDIRECT URI(S)"
configuration:

- https://YourHostNameHere/confirm_login
- https://YourHostNameHere/manage

#### Docker quickstart

If you have docker-compose installed it will offer the fastest route to getting
up-and-running.

```bash
git clone https://github.com/jonas747/yagpdb
cp yagpdb/yagpdb_docker/{app.example.env,app.env}
cp yagpdb/yagpdb_docker/{db.example.env,db.env}
```

Edit `app.env` and `db.env` to specify the Discord bot values from above.

Make sure ports 80 and 443 are accessible on your network and launch:

    docker-compose -f yagpdb/yagpdb_docker/docker-compose.yml up

The bot will connect automatically and the control panel will be available via
your host after a short setup.

If you are running several bots (or other web sites alongside the bot), consider
running a proxy such as jrcs/letsencrypt-nginx-proxy-companion.

First start the proxy. This needs to be started only once and is shared by all
web sites:

    docker network create proxy-tier
    docker-compose -p proxy yagpdb/yagpdb_docker/docker-compose.proxy.yml up

And then start the bot using the proxy:

    docker-compose -f yagpdb/yagpdb_docker/docker-compose.proxied.yml up

#### Manual setup

Required databases: 
 - PostgresSQL
     + Requires a db named yagpdb, this is currently hardcoded in.
     + The user and password is configurable through env variables.
 - Redis
     + Defaults are fine

First step is to set those up, and get them running.

**Steps for building:**

YAGPDB currently use a lot of alternative branches of my projects, mostly because of my custom discordgo fork.

```bash
git clone -b yagpdb https://github.com/jonas747/discordgo $GOPATH/src/github.com/jonas747/discordgo
git clone -b dgofork https://github.com/jonas747/dutil $GOPATH/src/github.com/jonas747/dutil
git clone -b dgofork https://github.com/jonas747/dshardmanager $GOPATH/src/github.com/jonas747/dshardmanager
git clone -b dgofork https://github.com/jonas747/dcmd $GOPATH/src/github.com/jonas747/dcmd
go get -v -d github.com/jonas747/yagpdb/cmd/yagpdb
cd $GOPATH/src/github.com/jonas747/yagpdb/cmd/yagpdb
go build -o yagpdb main.go
```
You can now run `./yagpdb`

Configuration is done through environment variables. See `cmd/yagpdb/sampleenvfile` for what environment variables are available.

You can run the webserver, bot, reddit and youtube parts as seperate processes (haven't tested on different physical machines yet, doubt it'll work well atm for the webserver and bot at least)

You specify `-bot` to run the bot, `-web` to run the webserver and so on.
And it should be running now.

The webserver by default (unless `-pa`) listens on 5000(http) and 5001(https)
So if you're behind a NAT, forward those, if not you can either use the `-pa` switch or add an entry to iptables.

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

[See the Issues Tab for more](https://github.com/jonas747/yagpdb/issues)

### Core packages:

- Web: The core webserver package, in charge of authentication.
- Bot: Core bot package, delegates events to plugins.
- Common: Handles all the common stuff between web and bot (config, discord session, redis pool etc).
- Feeds: Handles all feeds, personally I like to run each feed as its own service, that way I can start and stop individual feeds without taking everything down.
- Commands: Handles all commands.

Currently YAGPDB builds everything into one executable and you run the bot with the -bot switch, the webserver with the -web switch and so on (see -h for help).

### Databases

YAGPDB uses redis for light data and caching (and some remnants of configuration).

It uses postgresql for most configuration and more heavy data (logs and such).

### Contributing new features

See bot/plugin for info about bot plugins, web/plugin for web plugins and feeds/plugin for feeds if you wanna make a new fully fledged plugin.

Expect web, bot and feed instances to be run separately.

For basic utility/fun commands you can just jam them in stdcommands, use the existing commands there as an example of how to add one.

**If you need any help finding things in the source or have any other questions, don't be afraid of messaging me**
