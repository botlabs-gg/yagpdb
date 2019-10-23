This is version 2 of YAGPDB automoderator, v1 was scrapped and is deprecated, this version has been made from scratch and its goal is to be much more flexible and capable.

Quick design brainstorm:
 - Servers can create several rulesets
 - Rulesets contains sets of rules
 - Rules contains a set of conditions that all need to be met and another set of effects that will be executed once all the conditions for the rule are met
 - Infringement counters can either be ruleset scoped, global scoped, or custom key scoped (also either ruleset or global scoped, per user or per channel etc as well maybe?)
 - With this you could have a stricter set of automod rules for new users, and and more soft one for "trusted" long time members
 - You should be able to toggle rulesets on with commands, so if you need to slow down a channel or a raid is in progress, you can employ very strict automod rules via a single command invocation

**Basic structure, with some example effects and conditions**

Ruleset settings:
 - enabled
 - general user specific filters
 - general channel specific filters

Ruleset Rules:
 - Rule 1
     + Conditions (They all need to match):
         * ex: Send x messages within x seconds
         * filter: general user specific filters
         * filter: general channel specific filters
     + Effects applied once the conditions for the rule is met:
         * Increment (ruleset based | global | custom key based "key here") infringement counter and apply proper punishments.
         * (Create channel logs | log message in channel x)
         * Delete message
         * Could also apply a direct mute here for example, without setting up punishments?
 - Rule 2
     + ...

Ruleset punishments:
 - Punishment 1
     + Conditions:
         *  (ruleset based | global | custom key based "key here") infringements within x, higher than or equal to x
         * general user specific filters
         * only apply once per x interval
         * punishment x
     + Punishments:
         * mute, kick, ban, lost role, etc...

General user specific filters:
 - role whitelist/blacklist
 - account/member age
 - bot?

General channel specific filters:
 - direct whitelist/blacklist
 - category whitelist/blacklist
