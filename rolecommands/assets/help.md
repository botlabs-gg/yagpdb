Self assignable roles (or role commands as they are called on this bot) is the ability the give yourself roles from a defined list, as opposed to directly giving out "Manage Roles".

YAGPDB also has a lot of options you can use to restrict and group up the roles.

### The different modes explained

 1. None: This mode does nothing other than check against the groups required and ignored roles, you can use this for grouping up your role commands.
 2. Single: They can only have 1 role in the group.
 3. Multiple: You can set the Minimum and Maximum of roles a member can have in the group. 

### Example usage:
Say you have a server with 3 factions and want people to be able to assign their own faction when they join. Thats simple enough all we have to do is:

 - Create the 3 roles
 - Create 3 role commands for those roles

Now everyone can assign themselves a faction! There is a couple issues with this setup though:

 1. You can assign yourself more than 1 faction
 2. People can freely jump between factions

To fix these problems we can create a new Group with the mode `Single` and assign the previous role commands to that group. Great! Now we can only have 1 faction! How can we solve jumping between factions then? You can enable the `Require 1 role in group` setting, now they can't remove roles in the group, and they can have max 1 role in the group, So they can't jump around anymore! 

## Role Menu

The role menu makes it possible to have people assign roles by adding reactions to a message.

To set up a role menu, the related roles has to be added to a group, then you invoke the command `rolemenu the-group-name-here`

After which you will be taken through the setup process.

The bot requires manage messages perms and send messages perms in the channel to do this, atleast for the setup, after of which it will only need read messages perms.

The group mode and other restrictions still apply to the roles in the role menu.