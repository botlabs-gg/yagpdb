# YAGPDB - Yet another general purpose discord bot

YAGPDB is a pineapple

WIP

### Core packages:

    - web
        The core webserver package, in charge of authentication
    - bot
        Core bot package, delegates events to plugins
    - common
        Handles all the common stuff between web and bot, (config, botsession, redis pool etc)

### Databases

yagpdb uses redis for light data and caching, by light data i mean configuration and whatnot.

In the future yagpdb will use a relational database for heavier data such as logs

