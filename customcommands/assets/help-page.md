Custom commands allow you to create your own commands, the custom command system in YAGPDB is somewhat complex and can be used for some advanced stuff, but once you start needing very complex and advanced operations you should really think about making standalone bot for it.

All custom commands has a trigger, this is what triggers the custom command, the different trigger types are:

 - **Command**: With this trigger, the message has to start with the command prefix for your server (`-` by default) followed by the actual trigger
 - **Starts with**: When a message starts with your trigger
 - **Contains**: When a message contains your trigger
 - **Exact match**: When a message equals your trigger
 - **Regex**: This trigger allows you to use a regex pattern, the regex engine used is [go's regexp](https://golang.org/pkg/regexp/), if `case sensitive` is not set, the case insensitive flag is added to the trigger for you


For more details, see the [templating docs](/docs/templates)