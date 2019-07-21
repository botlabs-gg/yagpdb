YAGPDB
================

### Yet another general purpose discord bot

YAGPDB is a multifunctional, modular Discord bot. It's modular in that plugins exist for the most part on their own (with exceptions to some lazy things in the main stylesheet), some plugins do however depend on other plugins (most plugins depend on the commands plugin, for example).

**Links**
 - [YAGPDB.xyz](http://yagpdb.xyz)
 - [For updates and support join my discord server](https://discord.gg/4udtcA5)
 - [The documentation of YAGPDB](https://docs.yagpdb.xyz/)

### Running YAGPDB yourself

Running this bot may seem challenging, and that's because I don't have time to make it easy to run for everyone, for the most part, it should run fine after the initial work has been done. Please view [this page](https://docs.yagpdb.xyz/others/self-hosting-with-docker) for more information.

#### Updating
Updating after v1 should migrate schemas automatically, but you should always take backups beforehand if things go wrong.

I will put breaking changes in the breaking_changes.md file, which you should always read before updating.

#### There's 2 ways of running this bot

1. Using Docker
2. Standalone

**I will not help with basic problems or how to do unrelated things (how to run it on startup for example), use Google, if those well written tutorials and articles confuse you, how the hell is a guy with English as a second language gonna be any better?**

(There's still a lot of contributing you can do without this though, such as writing docs, fixing my horrible typos and so on)

#### General Discord bot setup

Directions on creating an app and getting credentials may be found
[here](https://github.com/reactiflux/discord-irc/wiki/Creating-a-discord-bot-&-getting-a-token).
YAGPDB does not require you to authorize the bot: all of that will be handled
via the Control Panel.

In addition, you will need to add the following urls to the bot's "REDIRECT URI(S)" configuration:

- https://YourHostNameHere/confirm_login
- https://YourHostNameHere/manage

#### Docker quickstart

If you have docker-compose installed, it will offer the fastest route to getting
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

If you are running several bots (or other websites alongside the bot), consider
running a proxy such as jrcs/letsencrypt-nginx-proxy-companion.

First, start the proxy. This needs to be started only once and is shared by all
websites:

    docker network create proxy-tier
    docker-compose -p proxy yagpdb/yagpdb_docker/docker-compose.proxy.yml up

And then start the bot using the proxy:

    docker-compose -f yagpdb/yagpdb_docker/docker-compose.proxied.yml up

#### Standalone/Manual setup

**Requirements**
 - Fairly recent go version (1.11 or later, I use new features as soon as they're out, so watch out in breaking_changes)
 - PostgreSQL 9.6 or later
 - Redis version 3.x or later (maybe it's possible to get it working with earlier versions, however I'm not 100% sure)

I may update the bot at any point to require newer versions of any of these, so you should ALWAYS check breaking_changes before updating.

**First step** is to set those up and get them running:

 1. Install and configure redis and postgres with your desired settings (my DM's are not Google, you'll have to figure at least this much out on your own...)
 2. Create a user named `yagpdb` and database named `yagpdb` which the user `yagpdb` has write access to
 3. Update your env vars with the config (see example env file in `cmd/yagpdb/`)
 4. Done.

**Steps for building:**

YAGPDB currently uses a lot of alternative branches of my projects, mainly because I also use a discordgo fork with a lot of goodies in it (why not push my changes upstream? Cause a shit ton of breaking changes that would never get accepted)

I'm working towards making YAGPDB fully `go get ...`-able

```bash
git clone -b yagpdb https://github.com/jonas747/discordgo $GOPATH/src/github.com/jonas747/discordgo
git clone -b dgofork https://github.com/jonas747/dutil $GOPATH/src/github.com/jonas747/dutil
git clone -b dgofork https://github.com/jonas747/dshardmanager $GOPATH/src/github.com/jonas747/dshardmanager
go get -v -d github.com/jonas747/yagpdb/cmd/yagpdb
cd $GOPATH/src/github.com/jonas747/yagpdb/cmd/yagpdb
go build
```

After this, unless you wanna run it in testing mode using `YAGPDB_TESTING=yes` you have to run `cmd/yagpdb/copytemplates.sh` to copy all the plugin specific template files into the `cmd/yagpdb/templates/plugins` folder.

You can now run `./yagpdb`

Configuration is done through environment variables. See `cmd/yagpdb/sampleenvfile` for what environment variables are available.

You can run the web server, bot, reddit and youtube parts as separate processes (some smallish limitations require to to run it on the same machine, will likely be removed soon as I'm gonna need to work on horizontal scaling soon)

You specify `-bot` to run the bot, `-web` to run the web server, `-feeds "youtube,reddit"` to run the reddit and youtube feeds.

The web server by default (unless `-pa`) listens on 5000(http) and 5001(https)
So if you're behind a NAT, forward those, if not you can either use the `-pa` switch or add an entry to iptables.

### Plugins

**Standard plugins:**

* Youtube-Feed
* Stream-announcements
* Server Stats
* Soundboard
* Reputation
* Reminder
* Reddit-Feed
* Notifications
* Moderation
* Logs
* Custom commands
* And More!

**Planned plugins**

[See the Issues Tab for more](https://github.com/jonas747/yagpdb/issues)

### Core packages:

- Web: The core web server package, in charge of authentication.
- Bot: Core bot package, delegates events to plugins.
- Common: Handles all the common stuff between web and bot (config, discord session, redis pool etc).
- Feeds: Handles all feeds, personally I like to run each feed as its own service, that way I can start and stop individual feeds without taking everything down.
- Commands: Handles all commands.

Currently, YAGPDB builds everything into one executable and you run the bot with the -bot switch, the web server with the -web switch and so on (see -h for help).

### Databases

YAGPDB uses redis for light data and caching (and some remnants of configuration).

It uses PostgreSQL for most configuration and more heavy data (logs and such).

### Contributing new features

See bot/plugin for info about bot plugins, web/plugin for web plugins and feeds/plugin for feeds if you wanna make a new fully fledged plugin.

Expect web, bot and feed instances to be run separately.

For basic utility/fun commands, you can just jam them in stdcommands. Use the existing commands there as an example of how to add one.

**If you need any help finding things in the source or have any other questions, don't be afraid of messaging me.**
