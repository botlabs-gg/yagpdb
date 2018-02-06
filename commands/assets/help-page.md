### General:
Command | Aliases | Optional Args | Description
------- | ------- | ------------- | -----------
help | N/A | (command) | Shows help for all or one specific command.
invite | inv, i | N/A | Response with the bot website link for invitation.
info | N/A | N/A | Response with bot information.

### Tools:
Command | Aliases | Requiered Args | Optional Args | Description
------- | ------- | -------------- | ------------- | -----------
calc | c, calculate | (what to calculate) | N/A | Calculator 2+2=5
ping | N/A | N/A | N/A | Response with pong and pingtime.
stats | N/A | N/A | N/A | Shows server stats (only if public stats are enabled).
logs | ps, paste, pastebin, log | N/A | (count) | Creates a log of the channels messages (max. 100).
whois | whoami | N/A | (username) | Shows information about given member.
usernames | unames, un | N/A | (username) | Shows past usernames of given member.
nicknames | nn | N/A | (username) | Shows past nicknames of given member.
role | N/A | N/A | (rolename) | Give yourself a role or list all available roles.
remindme | remind | (time) (message) | N/A | Schedules a reminder. Example: `remindme 1h30min are you still alive?`.
reminders | N/A | N/A | N/A | List of your active reminders with an ID.
creminders | N/A | N/A | N/A | Lists reminders only in the current channel with an ID. Only members with `manage server` permissions can use this command.
delreminder | rmreminder | (ID) | N/A | Deletes the reminder with the given ID.

### Moderation:
Command | Aliases | Requiered Args | Optional Args | Description
------- | ------- | -------------- | ------------- | -----------
ban | N/A | (username) | (reason) | Bans given member.
kick | N/A | (username) | (reason) | Kicks given member.
mute | N/A | (username) | (minutes) (reason) | Mutes given member.
unmute | N/A | (username) | (reason) | Unmutes given member.
report | N/A | (username) (reason) | Reports given member.
clean | clear, cl | (count) | (username) | Cleans the chat.
reason | N/A | (ID) (reason) | N/A | Add/Edit modlog reason from given ID.
warn | N/A | (username) (reason) | N/A | Warn given member. Warnings are saved.
warnings | N/A | (username) | N/A | Lists warnings of given member with an ID.
editwarning | N/A | (ID) (Reason) | N/A | Edit given warning.
delwarning | dw | (ID) | N/A | Deletes the warning with the given ID.
clearwarnings | clw | (username) | N/A | Clears all warnings from given member.

### Misc/Fun:
Command | Aliases | Requiered Args | Optional Args | Description
------- | ------- | -------------- | ------------- | -----------
reverse | r, rev | (text) | N/A | Reverse the text given.
weather | w | (location) | N/A | Show the weather for the given location. Add ?m after the location for metric. Example: `w bergen?m`.
topic | N/A | N/A | N/A | Generates a chat topic.
currenttime | ctime, gettime | N/A | (timezone) (delta time in hours) | Shows UTC time.
catfact | cf, cat, catfacts | N/A | N/A | Catfacts. What else?!
advice | N/A | N/A | (for?) | Get a advice.
throw | N/A | N/A | (username) | Throws random stuff at nearby people or at the given member.
roll | N/A | N/A | (number of sides) | Roll a dice. Specify nothing for 6 siddes, or specify a number for max. sides.
topservers | N/A | N/A | N/A | Responds with the top 10 servers the bot is on.
customembed | ce | (json) | N/A | Creates an embed from what you give it in json form: https://discordapp.com/developers/docs/resources/channel#embed-object.
takerep | -, tr, trep | (username) | (count) | Takes away given number of rep from given member. Default number is 1.
giverep | +, gr, grep | (username) | (count) | Give given number of rep to given member. Default number is 1.
rep | N/A | N/A | (username) | Shows your or the given member current rep and rank.
toprep | N/A | N/A | (offset) | Shows top 15 rep members on the server.
sentiment | sent | N/A | (text) | Does sentiment analysys on the given text or your last 5 messages longer than 3 words.
8ball | N/A | (question) | N/A | Wisdom.
soundboard | sb | N/A | (soundname) | Play or list soundboard sounds.

### Debug:
Command | Aliases | Description
------- | ------- | -----------
yagstatus | status | Shows YAGPDB's status.
currentshard | cshard | Shows the current shard this server is on.
memberfetcher | memfetch | Shows the current status of the member fetcher.
roledbg | N/A | Debug autorole assignment.
