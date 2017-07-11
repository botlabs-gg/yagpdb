The go template engine is used for YAGPDB's custom commands and in various other places with custom messages.
This page aims to help you to get the most out of templates and custom commands.

Put {{"{{"}}...{{"}}"}} around the names like that: `{{"{{"}}.User.Username{{"}}"}}`

## Available template data:

### User

| Field | Description |
| --- | --- |
| `.User.Username` | The users username |
| `.User.ID` | The users id |
| `<@.User.ID>` | Example of creating a mention of the user |
| `.User.Discriminator` | The users discriminator | 
| `.User.Avatar` | The users avatar id |
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
| `Nick` | The nickname for this member |
| `Roles` | A list of role id's that the member has |

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

### Snippets
* `<@{{"{{"}}.User.ID{{"}}"}}>` Outputs a mention to the user with the user ID, doesn't output an ID
* `{{"{{"}}if in .Member.Roles "12312"{{"}}"}} This will only show if the member has the role with id "12312"{{"{{"}}end{{"}}"}}`
