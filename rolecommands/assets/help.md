Self assignable roles (or role commands as they are called on this bot) is the ability the give yourself roles from a defined list, as opposed to directly giving out "Manage Roles".

YAGPDB also has a lot of options you can use to restrict and group up the roles.

### Example usage:
Say you have a server with 3 factions and want people to be able to assign their own faction when they join. Thats simple enough all we have to do is:

 - Create the 3 rules
 - Create 3 role commands for those roles

Now everyone can assign themselves a faction! There is a couple issues with this setup though:

 1. You can assign yourself more than 1 faction
 2. People can freely jump between factions

To fix these problems we can create a new Group with the mode `Single` and assign the previous role commands to that group. Great! Now we can only have 1 faction! How can we solve jumping between factions then? You can add the 3 faction roles to the `Ignore Roles` of the group, so that if they already have one of the faction roles, they can't use any role commands of this group anymore!


### The different modes explained

 1. None: This mode does nothing other than check against the groups required and ignored roles, you can use this for grouping up your role commands.
 2. Single: They can only have 1 role in the group.
 3. Multiple: You can set the Minimum and Maximum of roles a member can have in the group. 