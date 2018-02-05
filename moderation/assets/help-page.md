The moderation features yagpdb provides are:

 - Modlog that logs all the moderation actions the bot takes 
    + this may be removed in the future in favor of audit log mirroring+filtering
 - Generated message logs on any mod action
    + For example: if you use the `-ban @someone being stupid` command then the bot will also log the last 100 messages in the channel the command was executed, even including deleted messages
 - Kick and ban commands
     + Although, these may also be removed in the future, or not, the generated message logs from the channel is still usefull.
 - Mute
     + Assigns the mute role for a temporary duration
     + You have to set up the mute role yourself
 - Clean/Clear
     + Removes last x messages, filtering by user if specified
 - Warnings
     + Assign someone warnings which are tracked by the bot, the automoderator uses this for rules without any punishments.
 - [An extensive automoderator, the docs for automod are located here](/docs/automoderator)