The go template engine is used for YAGPDB's custom commands.
This page aims to help you to get the most out of the custom commands feature.

Put {{"{{"}}...{{"}}"}} around the names like that: `{{"{{"}}.User.Username{{"}}"}}`

Available template data:

> type User
* `.User.Username` Outputs the username
* `.User.Avatar` Outputs the user avatar
* `.User.ID` Outputs the user ID
* `<@.User.ID>` Outputs a mention to the user with the user ID, doesn't output an ID
* `.User.Discriminator` Outputs the user discriminator
* `.User.Bot` Outputs true or false, if the user is a bot it will be `true` and if not then `false`

> type Guild/Server
* `.Guild.ID` Outputs the guild ID
* `.Guild.Name` Outputs the guild name
* `.Guild.Icon` Outputs the guild icon
* `.Guild.Region` Outputs the guild region
* `.Guild.AfkChannelID` Outputs the afk channel ID
* `.Guild.OwnerID` Outputs the owner ID
* `.Guild.JoinedAt` Outputs when a user first joined the guild
* `.Guild.AfkTimeout` Outputs the time when user gets moved into the afk channel while not active
* `.Guild.MemberCount` Outputs the number of users on a guild
* `.Guild.VerificationLevel` Outputs the requiered verification level for the guild
* `.Guild.EmbedEnabled` Outputs if embeds are enabled or not, true/false

> Snippets
* `<@{{"{{"}}.User.ID{{"}}"}}>` Outputs a mention to the user with the user ID, doesn't output an ID
