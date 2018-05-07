The go template engine is used for YAGPDB's custom commands and in various other places with custom messages.
This page aims to help you to get the most out of templates and custom commands.

Put {{"{{"}}...{{"}}"}} around the names like that: `{{"{{"}}.User.Username{{"}}"}}`

## Available template data:

### User

| Field | Description |
| --- | --- |
| `.User.Username` | The user's username |
| `.User.ID` | The user's id |
| `<@.User.ID>` | Example of creating a mention of the user |
| `.User.Discriminator` | The user's discriminator | 
| `.User.Avatar` | The user's avatar id |
| `.User.Bot` | True if the user is a bot | 

### Guild/Server

| Field | Description |
| --- | --- |
| `.Guild.ID` | Outputs the guild ID |
| `.Guild.Name` | Outputs the guild name |
| `.Guild.Icon` | Outputs the guild icon |
| `.Guild.Region` | Outputs the guild region |
| `.Guild.AfkChannelID` | Outputs the AFK channel ID |
| `.Guild.OwnerID` | Outputs the owner ID |
| `.Guild.JoinedAt` | Outputs when a user first joined the guild |
| `.Guild.AfkTimeout` | Outputs the time when user gets moved into the AFK channel while not active |
| `.Guild.MemberCount` | Outputs the number of users on a guild |
| `.Guild.VerificationLevel` | Outputs the required verification level for the guild |
| `.Guild.EmbedEnabled` | Outputs if embeds are enabled or not, true/false |

### Member
| Field | Description |
| --- | --- |
| `.Member.Nick` | The nickname for this member |
| `.Member.Roles` | A list of role id's that the member has |

### Functions
| Function | Description |
| --- | --- |
| `adjective` | Returns a random adjective |
| `dict key1 value1 key2 value2 etc` | Creates a dictionary (not many use cases yet) |
| `in list value` | Returns true if value is in list |
| `title string` | Returns string with the first letter of each word capitalized |
| `add x y` | Returns x + y |
| `seq start stop` | Creates a new array of integers, starting from start and ending at stop |
| `shuffle list` | returns a shuffled version of list |
| `joinStr str1 str2 str3` | Joins several strings into 1, useful for executing commands in templates |
| `randInt (stop, or start stop)` | Returns a random integer between 0 and stop, or start - stop if 2 args provided.  |
| `toString` | Converts something to a string |
| `toInt` | Converts something to a int |
| `toInt64` | Converts something to a int64 |
| `exec command command_args` | Executes a command in the users context, max 3 commands can be executed per template |
| `sendDM "message_here"` | Sends the user a DM, only 1 DM can be sent per template |
| `mentionEveryone` | Mentions everyone |
| `mentionHere` | Mentions here |
| `mentionRoleName "rolename"` | Mentions the first role found with the provided name (case insensitive) |
| `mentionRoleID roleid` | Mentions the role with the provided ID (use the listroles command for a list of role) |
| `hasRoleName "rolename"` | Returns true if the user has the role with the specified name (case insensitive) |
| `hasRoleID roleid` | Returns true if the user has the role with the specified ID (use the listroles command for a list of role) |
| `addRoleID roleid` | Add the role with the given id to the user that triggered the command (use the listroles command for a list of role) |
| `removeRoleID roleid` | Remove the role with the given id from the user that triggered the command (use the listroles command for a list of role) |
| `deleteResponse` | Deletes the response after 10 seconds |
| `deleteTrigger` | Deletes the trigger after 10 seconds |
| `addReactions "👍" "👎"` | Adds each emoji as a reaction to the message that triggered the command |


### Branching
| Case | Example |
| --- | --- |
| Basic if | `{{"{{"}}if (condition){{"}}"}}{{"{{"}}end{{"}}"}}`
| And  | `{{"{{"}}if and (cond1) (cond2) (cond3){{"}}"}}` |
| Or   | `{{"{{"}}if or (cond1) (cond2) (cond3){{"}}"}}` |
| Equals  | `{{"{{"}}if eq .Channel.ID "########"{{"}}"}}` |
| Not Equals  | `{{"{{"}}if ne .Channel.ID "#######"{{"}}"}}` |
| Less than | `{{"{{"}}if lt (len .Args) 5{{"}}"}}` |
| Greater Than  | `{{"{{"}}if gt (len .Args) 1{{"}}"}}` |
| Else if | `{{"{{"}}if (case statement}} {{"{{"}}else if (case statement}} {{"{{"}}end{{"}}"}}` |
| Else | `{{"{{"}}if (case statement}} {{"{{"}}else}} output here {{"{{"}}end{{"}}"}}` |


### Snippets
* `<@{{"{{"}}.User.ID{{"}}"}}>` Outputs a mention to the user with the user ID, doesn't output an ID
* `{{"{{"}}if hasRoleName "funrole"{{"}}"}} This will only show if the member has a role with name "funrole" {{"{{"}}end{{"}}"}}`
* `{{"{{"}}if gt (len .Args) 1{{"}}"}} {{"{{"}}index .Args 1{{"}}"}} {{"{{"}}end{{"}}"}}` Will display your first input if input was given 
* `{{"{{"}}if eq .Channel.ID “#######”{{"}}"}}` Will only show in Channel #####
* `{{"{{"}}if ne  .User.ID “#######”{{"}}"}}` Will ignore if user ID ##### uses command
* `{{"{{"}}$d := randInt 10{{"}}"}}` Store the random int into variable $d 
* `{{"{{"}}$d := randInt 10{{"}}"}}` Store the random int into variable $d 
* `{{"{{addReactions .CmdArgs}}"}}` Adds the emoji following a trigger as reactions

### How to get a role ID

Use the `listroles` command. 


