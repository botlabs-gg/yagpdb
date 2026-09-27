# Privileged Gateway Intents Application

Copy-paste answers for Discord's "Request Intents Review" form, covering the three privileged
intents YAGPDB uses: **Server Members**, **Presence**, and **Message Content**.

You apply once your bot can reach 10,000 users, and reapply every year. Replace every
`<PLACEHOLDER>`, and delete anything that is not true of your bot: the final checkbox certifies
your answers are accurate.

If you do not yet have a privacy policy and terms of service, start from
[PRIVACY_POLICY_TEMPLATE.md](PRIVACY_POLICY_TEMPLATE.md) and
[TERMS_OF_USE_TEMPLATE.md](TERMS_OF_USE_TEMPLATE.md). Your privacy policy must say you store
message content and state the 30-day retention, because answer 11 below says so. If the two
disagree, that is a rejection.


For Video link, use the below video as a reference **to create your own video.**

https://drive.google.com/drive/u/2/folders/1-YjDsA6g4vSfeQO9gGt5KupAfXFAegSp

## Answer key

| # | Field | Answer |
| --- | --- | --- |
| 1 | What does your application do? | [Text](#1-what-does-your-application-do) |
| 2 | Do you have a public Privacy Policy? | **Yes** |
| 3 | Where is your Privacy Policy available? | [Text](#3-where-is-your-privacy-policy-available) |
| 4 | Please share a link to your Privacy Policy | [Link](#4-please-share-a-link-to-your-privacy-policy) |
| 5 | Why do you need the Guild Members intent? | [Text](#5-why-do-you-need-the-guild-members-intent) |
| 6 | Server Members: screenshots / videos | [Links](#6-server-members-screenshots-and-videos) |
| 7 | Are you storing any API Data off-platform? | **No** |
| 8 | Why do you need the Guild Presences intent? | [Text](#8-why-do-you-need-the-guild-presences-intent) |
| 9 | Presence: screenshots / videos | [Links](#9-presence-screenshots-and-videos) |
| 10 | Can users opt-out of Presence tracking? | **Yes** |
| 11 | Can users opt-out of message content tracking? | **No** |
| 12 | Are you storing message content off-platform? | **Yes** |
| 13 | Are you storing message content for 30 days or less? | **Yes** |
| 14 | Will message content train ML or AI Models? | **No** |
| 15 | Why do you need the Message Content intent? | [Text](#15-why-do-you-need-the-message-content-intent) |
| 16 | Message Content: screenshots / videos | [Links](#16-message-content-screenshots-and-videos) |
| 17 | Acknowledgement | Tick once everything above is true |

| Placeholder | What goes there |
| --- | --- |
| `<VIDEO LINK>` | A lasting link to your walkthrough video |
| `<LINK>` | A lasting link to that individual screenshot or clip |
| `<INSERT LINK TO PRIVACY POLICY>` | Your published privacy policy |
| `<MENTION HOW USERS CAN REACH OUT TO YOU TO DELETE THIS DATA>` | The address or channel people use to ask for deletion |

Files uploaded to Discord expire, so host screenshots and video somewhere lasting.

---

## 1. What does your application do?

```text
YAGPDB (Yet Another General Purpose Discord Bot) is a free, open-source server management and
moderation bot. Server owners set it up through a website control panel, where every feature is a
separate module that stays switched off until an administrator turns it on.

What it does for a server:

- Automatic moderation. Staff write their own rules about what is not allowed in their community
  and what happens when someone breaks them: banned words, links to banned sites, shouting in
  capitals, spam and flooding, repeated messages, unsolicited invites to other servers, and scam
  or phishing links. On a match the bot can delete the message, warn, mute, time out, kick or ban
  the member, change their roles, or log it for staff, escalating on repeat offences.

- Manual moderation: ban, kick, mute, time out and warn, a record of past warnings, and a cleanup
  command for clearing a batch of unwanted messages.

- Message logs. Staff can save a conversation to review later, including deleted messages, so that
  someone cannot harass another member and then delete the evidence.

- Custom commands, which server owners write themselves in a simple built-in scripting language.

- Welcome and goodbye messages, automatic roles, self-service role menus, and a captcha gate for
  new arrivals.

- Stream announcements, telling a server when one of its members starts a livestream.

- Smaller features: reputation, reminders, support tickets, a soundboard, trivia, event signups,
  timezone conversion, and YouTube, Twitch, Reddit and RSS announcements.

Source code: https://github.com/botlabs-gg/yagpdb
Website: https://yagpdb.xyz
Documentation: https://help.yagpdb.xyz

Video showing all three intents in use: <VIDEO LINK>
```

## 2. Do you have a public Privacy Policy?

**Yes.**

## 3. Where is your Privacy Policy available?

Mention details of how users can read your privacy policy, it would be best if a link to it is added to the bots bio, if you cannot modify the source code to add your privacy policy

## 4. Please share a link to your Privacy Policy.

<INSERT LINK TO PRIVACY POLICY>



## 5. Why do you need the Guild Members intent?

```text
This intent is how Discord tells us that someone joined a server, left it, or had their roles or
nickname changed. Several of our most-used features do nothing without it.

1. Automatic roles. Owners choose a role new members get, either on arrival or after a set time in
   the server. We cannot give someone a role on arrival unless we are told they arrived.

2. Welcome and goodbye messages when someone joins or leaves, and an optional private welcome.

3. Verification. New arrivals can be held behind a captcha and only let in once they pass. This is
   a common defence against automated spam accounts and has to start the moment someone joins.

4. Making mutes stick. If a muted member leaves and rejoins we restore their muted state. Without
   this, anyone muted can leave and rejoin to undo their punishment, making muting useless.

5. Moderation rules about a member's name rather than their messages. Staff can block offensive
   nicknames and act on accounts that join with an advertisement or an invite link to another
   server in their display name. This is one of the most common ways spam accounts operate, and it
   happens the moment they join, before they have said anything.

6. Moderation commands. Looking a member up, banning, kicking, muting or warning them requires
   finding that member, including members who have not spoken recently. Staff can also add or
   remove a role across everyone at once, which needs the full member list.

The alternative would be asking Discord about every member of every server we are in, repeatedly.
At our size that is an enormous continuous volume of requests, and automatic roles and verification
would still react minutes late instead of instantly, defeating their purpose.

None of this member data is written to our database. Roles, nicknames, join dates and the member
events themselves are used to carry out the action and then discarded.
```

## 6. Server Members: screenshots and videos

```text
Walkthrough video: <VIDEO LINK>

1. The automatic role settings in the control panel: <LINK>
2. A new member joining and being given that role: <LINK>
3. A welcome message and a goodbye message posted in a channel: <LINK>
4. The member lookup command, showing join date and roles: <LINK>
5. A moderation rule reacting to a member's nickname or display name: <LINK>
6. A muted member rejoining and still being muted: <LINK>

Feature documentation:
- Automatic roles: https://help.yagpdb.xyz/docs/roles/autorole/
- Welcome and goodbye messages: https://help.yagpdb.xyz/docs/notifications/general/
- Verification: https://help.yagpdb.xyz/docs/moderation/verification/
- Moderation tools: https://help.yagpdb.xyz/docs/moderation/moderation-tools/
- Name-based rules: https://help.yagpdb.xyz/docs/moderation/advanced-automoderator/triggers/
- Applying a role to everyone: https://help.yagpdb.xyz/docs/roles/bulk-role/
```

![Automatic role settings](intents/01-autorole-settings.png)
![Automatic role being given](intents/02-autorole-in-action.png)
![Welcome and goodbye messages](intents/03-welcome-message.png)
![Member lookup](intents/04-member-lookup.png)
![Nickname moderation rule](intents/05-nickname-rule.png)
![Mute restored after rejoining](intents/06-mute-rejoin.png)

## 7. Are you storing any API Data off-platform (outside of Discord)?

**No.**

## 8. Why do you need the Guild Presences intent?

```text
We use this intent for one thing: announcing when a member of a server starts a livestream.

Server owners choose a channel and write an announcement. When a member goes live, we post it so
the community knows to come and watch, and we can give the member a role marking them as currently
live, which is removed as soon as the stream ends. Owners can require a member to hold a particular
role before their streams are announced, so only people who opted in are covered.

Discord only tells us someone is streaming through their presence. There is no other way for a bot
to find out that a member has gone live, so there is no version of this feature that works without
this intent.

There is one smaller, read-only use: when a moderator looks a member up, the bot shows what that
member is currently doing, such as the game they are playing or their custom status. This is read
at the moment the command runs and is not recorded.

Documentation for this feature:
- Stream announcements and the currently-live role:
  https://help.yagpdb.xyz/docs/notifications/streaming/
```

## 9. Presence: screenshots and videos

```text
Walkthrough video: <VIDEO LINK>

1. The stream announcement settings in the control panel: <LINK>
2. The announcement posted when a member goes live, with the member list visible beside it showing
   that member streaming: <LINK>
3. The "currently live" role being given while the stream runs, and taken away when it ends: <LINK>

Feature documentation:
- Stream announcements and the currently-live role:
  https://help.yagpdb.xyz/docs/notifications/streaming/
```

![Stream announcement settings](intents/07-stream-settings.png)
![Stream announcement posted](intents/08-stream-announcement.png)
![Live role given and removed](intents/09-live-role.png)

## 10. Can users opt-out of having their Presence data tracked?
**Yes.**

## 11. Can users opt-out of having their message content data tracked?
**NO.**

## 12. Are you storing message content data off-platform?
**Yes.**

## 13. Are you storing user message content data for 30 days or less?
**Yes**

### How do users contact you to request deletion of their activity data?

```text
Message content data is only stored for moderation events, other than that there is no message content tracking of any sort. 
All message logs can also be deleted anytime by a server admin / moderator from the dashboard of the bot. 
All message logs are also automatically deleted as soon as they become 30 days old. 

<MENTION HOW USERS CAN REACH OUT TO YOU TO DELETE THIS DATA>
```

## 14. Will the message content data be used to train machine learning or AI Models?

**NO**


## 15. Why do you need the Message Content intent?

```text
Message content is what our moderation and automation features work on. Without it, automatic
moderation, message logs and a large share of our servers' custom commands stop working.

1. Automatic moderation, the main reason. Every rule staff write is a question about what a message
   says: banned words, or an allow-list of permitted words for strictly moderated servers; links to
   banned sites, or an approved-site list; patterns staff write themselves, for spam that mutates
   to evade word filters; messages in all capitals; flooding, counted per member and per channel;
   the same message repeated; unsolicited invites to other servers, the most common spam we see;
   and scam and phishing links. A match applies the server's chosen response: delete, warn, mute,
   time out, kick, ban, change roles, or log for staff, escalating on repeat offences.

   Discord's AutoMod covers part of this, but not staff-written patterns, duplicate detection
   across channels, our scam-link protection, allow-list filtering, or our escalation model.

2. Message logs. To investigate a harassment report staff need to see what was said, and the
   message has usually been deleted by its author. We also attach a short snapshot of the
   surrounding conversation to a ban, kick, mute or warning, so the reason for the action is kept
   with the record.

3. Cleanup. Removing messages that match a word or pattern means reading them.

4. Custom commands. Many Types respond to what members say and use advanced pattern matching like Regex, Message Contains, Message Starts, and people have built 100s of advanced use cases based on this, here is a community maintained list of some advanced custom commands https://yagpdb-cc.github.io/

5. Reputation when one member thanks another, timezone conversion, and event signups.

Where an alternative existed we already moved off message content: built-in commands are slash only now, and the text prefix no longer runs them.
```

## 16. Message Content: screenshots and videos

```text
Walkthrough video: <VIDEO LINK>

1. The moderation rule builder, showing a rule about message content: <LINK>
2. That rule deleting a message, and the matching entry in the staff log: <LINK>
3. A scam link being blocked: <LINK>
4. Saved message logs, in chat and in the web view: <LINK>
5. The logging privacy settings: excluded channels, which roles may read logs, and who can see
   deleted messages: <LINK>
6. A custom command responding to what someone said: <LINK>
7. The cleanup command removing a batch of matching messages: <LINK>

Feature documentation:
- Automatic moderation: https://help.yagpdb.xyz/docs/moderation/advanced-automoderator/overview/
- Rule triggers: https://help.yagpdb.xyz/docs/moderation/advanced-automoderator/triggers/
- Rule conditions, including the exemptions servers can set:
  https://help.yagpdb.xyz/docs/moderation/advanced-automoderator/conditions/
- Rule effects: https://help.yagpdb.xyz/docs/moderation/advanced-automoderator/effects/
- Message logs, retention and who can view them: https://help.yagpdb.xyz/docs/moderation/logging/
- Cleanup and other moderation tools: https://help.yagpdb.xyz/docs/moderation/moderation-tools/
- Custom commands and their triggers: https://help.yagpdb.xyz/docs/custom-commands/commands/
- Reputation: https://help.yagpdb.xyz/docs/fun/reputation/
```

![Moderation rule builder](intents/10-rule-builder.png)
![Rule deleting a message](intents/11-rule-in-action.png)
![Scam link blocked](intents/12-scam-link-blocked.png)
![Saved message logs](intents/13-message-logs.png)
![Logging privacy settings](intents/14-log-privacy-settings.png)
![Custom command responding to a message](intents/15-custom-command.png)
![Cleanup command](intents/16-cleanup.png)

---

## 17. Acknowledgement

Tick once everything above is true of the bot you are running.