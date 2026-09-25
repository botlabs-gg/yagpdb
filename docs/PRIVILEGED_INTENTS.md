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

## Answer key

| # | Field | Answer |
| --- | --- | --- |
| 1 | What does your application do? | [Text](#1-what-does-your-application-do) |
| 2 | Do you have a public Privacy Policy? | **Yes** |
| 3 | Why do you need the Guild Members intent? | [Text](#3-why-do-you-need-the-guild-members-intent) |
| 4 | Server Members: screenshots / videos | [Links](#4-server-members-screenshots-and-videos) |
| 5 | Are you storing any API Data off-platform? | **Yes** |
| 6 | Why do you need the Guild Presences intent? | [Text](#6-why-do-you-need-the-guild-presences-intent) |
| 7 | Presence: screenshots / videos | [Links](#7-presence-screenshots-and-videos) |
| 8 | Can users opt-out of Presence tracking? | **Yes** |
| 9 | Are you storing user activity data off-platform? | **No** |
| 10 | Can users opt-out of message content tracking? | **Yes** |
| 11 | Are you storing message content off-platform? | **Yes** |
| 12 | Will message content train ML or AI Models? | **No** |
| 13 | Why do you need the Message Content intent? | [Text](#13-why-do-you-need-the-message-content-intent) |
| 14 | Message Content: screenshots / videos | [Links](#14-message-content-screenshots-and-videos) |
| 15 | Acknowledgement | Tick once everything above is true |

| Placeholder | What goes there |
| --- | --- |
| `<PRIVACY POLICY URL>` | Public link, also added in the Developer Portal |
| `<TOS URL>` | Public link, also added in the Developer Portal |
| `<N>` | Roughly how many servers your bot is in |
| `<VIDEO LINK>` | A lasting link to your walkthrough video |
| `<LINK>` | A lasting link to that individual screenshot or clip |

Files uploaded to Discord expire, so host screenshots and video somewhere lasting.

---

## 1. What does your application do?

```text
YAGPDB (Yet Another General Purpose Discord Bot) is a free, open-source server management and
moderation bot. Server owners set it up through a website control panel, where every feature is a
separate module that stays switched off until an administrator turns it on.

What it does for a server:

- Automatic moderation. Staff write their own rules about what is not allowed in their community
  and what should happen when someone breaks them. Rules can look for banned words, links to banned
  websites, shouting in capital letters, spam and flooding, repeated identical messages,
  unsolicited invites to other servers, and scam or phishing links. When a rule matches, the bot
  can delete the message, warn, mute, time out, kick or ban the member, change their roles, or note
  it in the staff log, with repeat offences escalating automatically.

- Manual moderation tools: ban, kick, mute, time out and warn, a record of past warnings, and a
  cleanup command for clearing a batch of unwanted messages.

- Message logs. Staff can save a copy of a conversation to review later, including a record of
  deleted messages, so that someone cannot harass another member and then delete the evidence.

- Custom commands, which server owners write themselves using a simple built-in scripting language
  instead of having to host a bot of their own.

- Welcome and goodbye messages, automatic roles for new members, self-service role menus, and a
  captcha gate for new arrivals.

- Stream announcements, telling a server when one of its members starts a livestream.

- Smaller community features: reputation points, reminders, support tickets, a soundboard, trivia,
  event signups, timezone conversion, and announcements for new YouTube, Twitch, Reddit and RSS
  posts.

Source code: https://github.com/botlabs-gg/yagpdb
Website: https://yagpdb.xyz
Documentation: https://help.yagpdb.xyz
Privacy policy: <PRIVACY POLICY URL>
Terms of service: <TOS URL>
Currently in roughly <N> servers.

Video showing all three intents in use: <VIDEO LINK>
```

## 2. Do you have a public Privacy Policy?

**Yes.**

---

## 3. Why do you need the Guild Members intent?

```text
This intent is what lets Discord tell us when someone joins a server, leaves it, or has their roles
or nickname changed. Several of the bot's most-used features do nothing at all without it.

1. Automatic roles. Server owners choose a role new members should get, either the moment they
   join or after they have been around for a set amount of time. We cannot give someone a role on
   arrival unless Discord tells us they have arrived.

2. Welcome and goodbye messages, posted when someone joins or leaves, and an optional private
   welcome message to the new member.

3. Verification. New arrivals can be held behind a captcha and only let into the server once they
   pass it. This is a common defence against automated spam accounts, and it has to start the
   moment someone joins.

4. Making mutes stick. If a muted member leaves and comes back, we restore their muted state.
   Without this, anyone who is muted can simply leave and rejoin to undo their punishment, which
   makes muting close to useless.

5. Moderation rules about a member's name rather than their messages. Staff can block offensive
   nicknames, and automatically act on accounts that join with an advertisement or an invite link
   to another server in their display name. This is one of the most common ways spam accounts
   operate, and it happens the moment they join, before they have said anything.

6. Moderation commands. Looking up a member, banning, kicking, muting or warning them all require
   us to find that member, including members who have not spoken recently. Staff can also add or
   remove a role across everyone in the server at once, which needs the full member list.

The only alternative would be for the bot to repeatedly ask Discord about every member of every
server it is in. At our size that would mean an enormous, continuous volume of requests to Discord,
and features like automatic roles and verification would still react minutes late instead of
instantly, which defeats their purpose.

Documentation for the features described above:
- Automatic roles: https://help.yagpdb.xyz/docs/roles/autorole/
- Welcome and goodbye messages: https://help.yagpdb.xyz/docs/notifications/general/
- Verification: https://help.yagpdb.xyz/docs/moderation/verification/
- Moderation tools: https://help.yagpdb.xyz/docs/moderation/moderation-tools/
- Name-based moderation rules: https://help.yagpdb.xyz/docs/moderation/advanced-automoderator/triggers/
- Applying a role to everyone: https://help.yagpdb.xyz/docs/roles/bulk-role/
```

## 4. Server Members: screenshots and videos

```text
Walkthrough video: <VIDEO LINK>

1. The automatic role settings in the control panel: <LINK>
2. A new member joining and being given that role: <LINK>
3. A welcome message and a goodbye message posted in a channel: <LINK>
4. The member lookup command, showing join date and roles: <LINK>
5. A moderation rule reacting to a member's nickname or display name: <LINK>
6. A muted member rejoining and still being muted: <LINK>
```

![Automatic role settings](intents/01-autorole-settings.png)
![Automatic role being given](intents/02-autorole-in-action.png)
![Welcome and goodbye messages](intents/03-welcome-message.png)
![Member lookup](intents/04-member-lookup.png)
![Nickname moderation rule](intents/05-nickname-rule.png)
![Mute restored after rejoining](intents/06-mute-rejoin.png)

## 5. Are you storing any API Data off-platform (outside of Discord)?

**Yes.**

```text
Yes, but only member IDs, and only where a server's own settings and moderation history need them:
warnings, mutes, bans, and punishments scheduled to expire later. A server's settings also
reference the IDs of members exempted from a rule or given a particular permission. These are kept
until server staff delete them. Removing the bot from a server does not by itself erase them, so a
server that adds the bot again still has its settings and history; a server owner can ask us to
delete everything we hold for their server.

We keep nothing else about a member. Details such as roles and nicknames are held only in the bot's
temporary working memory so it can answer commands quickly, are discarded once unused for a while,
and do not survive the bot restarting.

We do not sell member information, share it with anyone else, or use it to train any kind of model.
```

---

## 6. Why do you need the Guild Presences intent?

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

We do not keep presence information. It exists only in the bot's temporary working memory, is
discarded automatically, and is never written to our database. We do not build profiles of what
members do, we do not measure time spent in games, and nothing in the product shows a member's past
activity.

Documentation for this feature:
- Stream announcements and the currently-live role:
  https://help.yagpdb.xyz/docs/notifications/streaming/
```

## 7. Presence: screenshots and videos

```text
Walkthrough video: <VIDEO LINK>

1. The stream announcement settings in the control panel: <LINK>
2. The announcement posted when a member goes live, with the member list visible beside it showing
   that member streaming: <LINK>
3. The "currently live" role being given while the stream runs, and taken away when it ends: <LINK>
```

![Stream announcement settings](intents/07-stream-settings.png)
![Stream announcement posted](intents/08-stream-announcement.png)
![Live role given and removed](intents/09-live-role.png)

## 8. Can users opt-out of having their Presence data tracked?

**Yes.**

```text
Yes, in three ways:

1. Through Discord's own settings. A member who appears offline, or who turns off activity status
   sharing, produces nothing for us to react to and the feature simply does nothing for them.
2. By not streaming. The feature only acts on members actively livestreaming with a public stream
   link. Owners can also require a specific role before streams are announced, so announcements
   only cover people who chose to take that role.
3. At the server level. Stream announcements are off by default and must be deliberately turned on
   by an administrator.

Because we never store presence information, there is no saved data for anyone to ask us to delete.
```

## 9. Are you storing user activity data off-platform?

**No.**

```text
No. Presence and activity information is held only in the bot's temporary working memory, is
discarded automatically, and is never written to our database or to disk. None of it survives the
bot restarting.

The only lasting trace of the stream announcement feature is a short list of who is currently live,
which exists purely so the same stream is not announced twice. It says nothing about what anyone is
doing, and a member is removed from it the moment their stream ends.
```

---

## 10. Can users opt-out of having their message content data tracked?

**Yes.**

```text
Yes, through the controls available to servers and their members:

- Every feature that reads messages is off by default and must be deliberately turned on by an
  administrator.
- Message logging can be limited by channel, either by excluding particular channels or by
  restricting logging to an approved list, so a server can set aside spaces that are never logged.
- Moderation rules can be set to ignore particular channels, whole categories, threads, bots, and
  members holding a given role, so individual people and entire areas of a server can be exempted.
- Saved logs are access-controlled. Administrators choose which roles may read them, and whether
  deleted messages are visible to anyone who is not a moderator.
- Server staff can delete saved logs at any time.
- Anyone who does not want their messages processed can ask that server's administrators to exclude
  the channels they use or exempt a role they hold.
```

> YAGPDB has no single switch that opts an individual member out everywhere. The controls above
> belong to servers and their administrators, so describe them that way. Claiming a personal opt-out
> the bot does not have will fail the review, or lose the intent later when someone checks.

## 11. Are you storing message content data off-platform?

**Yes.**

```text
Yes, for two features, both off until a server turns them on:

1. Message logs. We store the text of messages, with who wrote them and when, in three cases:

   - When a moderation action is taken. If a moderator bans, kicks, mutes or warns someone, or a
     member is reported, we save a short snapshot of the surrounding conversation, currently the
     last 100 messages in that channel, and attach it to the moderation record. This is the
     context of why the action was taken, so that staff reviewing the decision later, or handling
     an appeal, can see what actually happened rather than just a reason someone typed. It is
     bounded to that one channel at that one moment, not continuous collection.
   - When a moderator saves a conversation on purpose, to investigate a report.
   - Where a server has enabled deleted-message logging.

   Storing the text is the point of the feature: it exists so staff can still see what was said
   after a message has been deleted or edited. In all three cases a server's excluded channels are
   respected, and no log is created from a channel the server has set aside.

2. Support ticket records. When a ticket is closed, the conversation is saved so staff have a
   record of it, either posted into a channel the server chooses or attached as a text file.

Message logs are deleted automatically once they are 30 days old, whatever else happens. Staff can
also delete them sooner at any time, and a server owner can ask us to delete everything we hold for
their server.

Everything else that reads messages looks at them and forgets them. Automatic moderation checks a
message against the server's rules and keeps no copy; if a rule is broken we record which rule and
which member, not what was said. Command handling, custom commands, reputation and timezone
conversion all read the message, act, and discard it.
```

## 12. Will the message content data be used to train machine learning or AI Models?

**No.**

```text
Message content is never used to train or test any machine learning or AI model, and is never sold
or shared with anyone for that purpose.
```

## 13. Why do you need the Message Content intent?

```text
Message content is what the bot's moderation and automation features work on. Without it the
automatic moderation system, message logs, and a large share of the custom commands our servers
have written stop working completely.

1. Automatic moderation, the main reason. Staff write their own rules about what is not allowed in
   their community, and every rule is a question about what a message says:
   - Banned words, or conversely a list of the only words permitted, used by heavily moderated and
     child-friendly servers.
   - Links to banned websites, or only permitting an approved list of sites.
   - Patterns staff describe themselves, for spam that changes shape to evade simple word filters.
   - Messages written entirely in capital letters.
   - Flooding: too many messages, links, images or mentions in a short time, measured both per
     member and per channel.
   - The same message posted over and over.
   - Unsolicited invites to other servers, including through third-party server listing sites. This
     is the most common form of spam our servers deal with.
   - Scam and phishing links, matched against lists of known malicious addresses.

   Discord's own AutoMod covers part of this, but not the pattern-based rules, repeated messages
   across different channels, our scam-link protection, the approved-list filtering strict servers
   depend on, or the escalation system our rules are built around.

2. Message logs and deleted-message records. When a member reports harassment, staff need to see
   what was actually said, and very often the offending message has already been deleted by the
   person who sent it. This is the main tool servers use to investigate reports fairly.

   The same applies to the bot's own moderation actions. When a member is banned, kicked, muted,
   warned or reported, we attach a short snapshot of the surrounding conversation to the record, so
   the reason for the action is preserved alongside it. Without message content a moderation record
   is just a name and a sentence someone typed, which is very hard to review or appeal fairly.

3. Cleanup. The cleanup command removes messages matching a word or pattern, for example clearing
   up after a spam wave, which means reading the messages it is deciding about.

4. Custom commands. Server owners write their own commands, and a large proportion are set to
   respond to what people say rather than to a slash command, for example answering a common
   question whenever it is asked, or reacting to a phrase in conversation. The command is also
   given the text that triggered it so it can respond appropriately. These triggers are chosen by
   the server, in the server's own wording, so there is no fixed command name to register as a
   slash command instead. Removing this would silently break many thousands of commands our
   servers already rely on.

5. Smaller features that respond to conversation: awarding reputation when one member thanks
   another, converting times mentioned in chat into each reader's timezone, and event signups.

Where an alternative existed, we have already moved off message content. The bot's own built-in
commands used to be available by typing a text prefix; they are now slash commands only, and the
prefix no longer works for them. We removed that use of message content deliberately, because
slash commands do the same job without it. What remains in the list above are the features where
there is no equivalent, because they have to react to what a member chose to say rather than to a
command aimed at the bot.

Documentation for the features described above:
- Automatic moderation, overview: https://help.yagpdb.xyz/docs/moderation/advanced-automoderator/overview/
- The full list of rule triggers: https://help.yagpdb.xyz/docs/moderation/advanced-automoderator/triggers/
- Rule conditions, including the exemptions servers can set:
  https://help.yagpdb.xyz/docs/moderation/advanced-automoderator/conditions/
- What a rule can do when it matches: https://help.yagpdb.xyz/docs/moderation/advanced-automoderator/effects/
- Message logs, retention and who can view them: https://help.yagpdb.xyz/docs/moderation/logging/
- Cleanup and other moderation tools: https://help.yagpdb.xyz/docs/moderation/moderation-tools/
- Custom commands and their triggers: https://help.yagpdb.xyz/docs/custom-commands/commands/
- Reputation: https://help.yagpdb.xyz/docs/fun/reputation/
```

## 14. Message Content: screenshots and videos

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
```

![Moderation rule builder](intents/10-rule-builder.png)
![Rule deleting a message](intents/11-rule-in-action.png)
![Scam link blocked](intents/12-scam-link-blocked.png)
![Saved message logs](intents/13-message-logs.png)
![Logging privacy settings](intents/14-log-privacy-settings.png)
![Custom command responding to a message](intents/15-custom-command.png)
![Cleanup command](intents/16-cleanup.png)

---

## 15. Acknowledgement

Tick once everything above is true of the bot you are running.
