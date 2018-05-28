The go template engine is used for YAGPDB's custom commands and in various other places with custom messages.
This page aims to help you to get the most out of templates and custom commands.

Put {{"{{"}}...{{"}}"}} around the names like that: `{{"{{"}}.User.Username{{"}}"}}`

### Quick intro

Invoking functions: `{{"{{"}}add 1 2{{"}}"}} = 3?`

Printing variables: `Hello {{"{{"}}.User.Username{{"}}"}}!`
    
Assigning variables: ` {{"{{"}}$stupidUser := .User{{"}}"}}!`

Note: Variables assigned in the scope block will be forgotten once `{{"{{"}}end{{"}}"}}` is reached.

For branching (if statements and such) look below.

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
| `deleteResponse seconds-delay` | Deletes the response after 10 seconds, or the specified delay |
| `deleteTrigger seconds-delay` | Deletes the trigger after 10 seconds, or the specified delay |
| `addReactions "👍" "👎"` | Adds each emoji as a reaction to the message that triggered the command |
| `userArg userID or mentionstring` | Returns the user object for the specified user, meant to be used with exec and execAdmin |
| `exec command arguments...` | Runs a command, this is quite advanced, see below for more information |
| `execAdmin command arguments...` | Runs a command as if the user that triggered it is the bot, this is quite advanced, see below for more information |
| `slice string-or-slice start [end]` | Slices the the provided slice into a subslice that starts at start index and ends at end index (or the rest of the provided slice if no end index is provided), example: `slice .CmdArgs 1`: creates a slice of the cmdargs excluding the first element |


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

### exec and execAdmin functions

The format is `exec/execAdmin command argument1 argument2...`

Accepted arguments are: numbers, strings, string arrays/slices and also User objects!

I'll start off with an example:

`{{"{{"}}exec "reverse" "123" {{"}}"}}`

Lets break it down:

 - exec: the function
 - "reverse": the command were executing
 - "123": the argument were providing to the command

This executes the `reverse` command, and puts the output in it's place, in the above example, it would be replaced with `321`

It would be the same as sending `-reverse 123` in the chat.


A more complicated example:

`{{"{{"}}exec "giverep" (userArg 232658301714825217) {{"}}"}}`

This will make the user who triggered the command give rep to the user with the id above. 
The `userArg` function can be used to retrieve a user from a mention string or id.

**execAdmin**: 

execAdmin executes commands as if the bot was the one invoking them, meaning if someone without kick permissions triggered a custom command, and that custom command used `execAdmin` with the kick command, it would go through fine.

Example: Say you want to mute everyone that triggers a custom command for 1 minute:

`{{"{{"}}execAdmin "mute" .User 1 "You triggered the evil custom command" {{"}}"}}`

The only new thing here is were using execAdmin instead of exec, this works even if the user that triggers it dosen't have admin.