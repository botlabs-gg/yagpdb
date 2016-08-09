# YAGPDB - Yet another general purpose discord bot

YAGPDB is a pineapple

### Project status

YAGPDB is very much work in progress so i'm not actively advertising it. Although i do have it perform a lot of duties on a somewhat large server, but it's still in alpha so beware of bugs.

**Links**
 - [YAGPDB.xyz](http://yagpdb.xyz)
 - [For updates and support join my discord server](https://discord.gg/Cj6kCba)

The naming styles of template information used to be in underscore but im in the process of changing it to the same as go's (CamelCase).  

### Core packages:

    - web
        The core webserver package, in charge of authentication
    - bot
        Core bot package, delegates events to plugins
    - common
        Handles all the common stuff between web and bot, (config, botsession, redis pool etc)

Currently YAGPDB builds everything into one executable and you run say the bot with the -bot switch, the webserver with the -web switch and so on (see -h for help)

### Databases

yagpdb uses redis for light data and caching, by light data i mean configuration and whatnot.

In the future yagpdb will use a relational database for heavier data such as logs

