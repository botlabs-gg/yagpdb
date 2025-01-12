# YAGPDB - Yet Another General Purpose Discord Bot

YAGPDB is a multifunctional, modular Discord bot. It is modular in the sense that for most things plugins exist -- However, some plugins may depend on other plugins.

## Plugins

* YouTube Feed
* Stream Announcements
* Server Stats
* Soundboard
* Reputation
* Reminders
* Reddit Feed
* Notifications
* Moderation
* Logs
* Custom Commands
* And More!

## Useful Links

* [Homepage](https://yagpdb.xyz)
* [Support Server](https://discord.gg/4udtcA5)
* [Help Center](https://help.yagpdb.xyz)

## Selfhosting

There are two ways of selfhosting this bot: [standalone](#Hosting-Standalone), or [dockerized](#Hosting-Dockerized).

### General Bot Setup

Directions on creating an app and getting credentials may be found
[here](https://github.com/reactiflux/discord-irc/wiki/Creating-a-discord-bot-&-getting-a-token).

YAGPDB does not require you to authorize the bot: all of that will be handled
via the Control Panel.

In addition, you will need to add the following urls to the bot's "REDIRECT URI(S)" configuration:

* <https://YourHostNameHere/confirm_login>
* <https://YourHostNameHere/manage>

### Hosting Dockerized

If you have docker-compose installed, that might be the fastest route of getting the bot up and running:

```shell
git clone https://github.com/botlabs-gg/yagpdb
cp yagpdb/yagpdb_docker/{app.example.env,app.env}
cp yagpdb/yagpdb_docker/{db.example.env,db.env}
```

Edit both env files accordingly. Make sure ports 80 and 443 are accessible on your network and that you have a proper image in `docker-compose.yml`:

```shell
docker-compose -f yagpdb/yagpdb_docker/docker-compose.yml up
```

Alternatively, you can run the bot behind a proxy:

```shell
docker network create proxy-tier
docker-compose -p proxy yagpdb/yagpdb_docker/docker-compose.proxy.yml up
docker-compose -f yagpdb/yagpdb_docker/docker-compose.proxied.yml up
```

During development, use the `docker-compose.dev.yml` file:

```shell
docker-compose -f yagpdb/yagpdb_docker/docker-compose.dev.yml up
```

### Hosting Standalone

#### Requirements

* Golang 1.23 or above
* PostgreSQL 9.6 or later
* Redis version 5.x or later

#### Setting Up

Configure Redis and Postgres with your desired settings.

In postgres, create a new user `yagpdb` and database `yagpdb` and grant that user access to that database.

Set up the environment variables with the credentials from the [general setup](#General-Bot-Setup). See the [sample env file](cmd/yagpdb/sampleenvfile) for a list of all enviroment variables.

Afterwards, run the build script located at `/cmd/yagpdb/build.sh` and  start the bot using `./yagpdb`:

```shell
git clone https://github.com/botlabs-gg/yagpdb
cd yagpdb/cmd/yagpdb
sh build.sh
./yagpdb -all
```

See `./yagpdb -help` for all usable run flags. The webserver listens by default on ports 5000 (HTTP) and 5001 (HTTPS).

## Databases

YAGPDB uses Redis for light data and caching, and postgresql for most configurations and heavy data, such as logs.

### Updating

Updating with v1 and higher should migrate schemas automatically, but you should always make backups.

Breaking changes can be found in breaking_changes.md, which should always be consulted before updating.

## Contributing

Please view the [contributing guidelines](CONTRIBUTING.md) before submitting any contributions.

See bot/plugin for info about bot plugins, web/plugin for web plugins and feeds/plugin for feeds if you wanna make a new fully fledged plugin.

Expect web, bot and feed instances to be run separately.

For basic utility/fun commands, you can just jam them in stdcommands. Use the existing commands there as an example of how to add one.

Please check CONTRIBUTING.md for further details.
