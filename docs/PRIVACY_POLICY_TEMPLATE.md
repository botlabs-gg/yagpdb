# Privacy Policy Template

A starting point for a YAGPDB instance's privacy policy. It is written to match the answers in
[PRIVILEGED_INTENTS.md](PRIVILEGED_INTENTS.md), so that a Discord reviewer who reads your form
answers and then opens your policy sees the same story twice. If you change one, change the other.

**This is not legal advice.** It is a description of what the software does, shaped into the form a
privacy policy usually takes. Have a lawyer review it if your instance is large, commercial, or
operating somewhere with strict privacy law.

## Before you publish

1. Replace every `<PLACEHOLDER>`.
2. Work through the **Optional sections** at the bottom. Each one covers a feature that is off by
   default. Paste in the ones you have switched on, delete the rest.
3. Decide whether to promise a response time for data requests, and add it to the "How to have
   your information deleted" section if you do. This template does not state one, because
   promising a deadline you do not meet is worse than not promising one. Note that if you handle
   data about people in the UK or EU, data protection law generally requires you to respond within
   one month whether or not your policy says so.
4. Put a real date on it and keep it updated.
5. Link it in the Discord Developer Portal, and somewhere obvious on your control panel.

| Placeholder | What goes there |
| --- | --- |
| `<BOT NAME>` | What your instance is called |
| `<OPERATOR>` | The person or organisation running it |
| `<CONTACT EMAIL>` | A monitored address |
| `<SUPPORT SERVER INVITE>` | Where people can reach you |
| `<WEBSITE>` | Your control panel address |
| `<COUNTRY OR REGION>` | Where your servers are hosted |
| `<DATE>` | Today |
| `<PAYMENT PROVIDER>` | Only in the optional premium section |
| `<HOW LONG>` | Only in the optional verification section |

---

# Privacy Policy

**Last updated: `<DATE>`**

This policy explains what `<BOT NAME>` does with information when you use it on Discord or sign in
to its control panel at `<WEBSITE>`. `<BOT NAME>` is operated by `<OPERATOR>`. You can reach us at
`<CONTACT EMAIL>` or in our support server at `<SUPPORT SERVER INVITE>`.

`<BOT NAME>` is a server management and moderation bot. Almost everything it does is switched off
until an administrator of a Discord server turns it on, and what it collects in any given server
depends on which features that server's administrators have chosen to use.

## The short version

| What | Do we store it? | For how long |
| --- | --- | --- |
| A server's settings | Yes | Until deleted by the server's staff or on request |
| Moderation records: warnings, mutes, bans | Yes | Until the server's staff delete them |
| Saved message logs | Yes | Deleted automatically after 30 days |
| Support ticket records | Yes | Decided by each server |
| Your roles, nickname and other member details | No | Held only in temporary memory |
| Your online status and activity | No | Held only in temporary memory |
| Control panel sign-in session | Yes | Until you sign out or it expires |

We do not sell your information, we do not use it for advertising, and we do not use it to train
any machine learning or AI model.

## What information the bot receives

In a server where `<BOT NAME>` is present, Discord sends it:

- **Your membership of that server.** Your user ID, username and display name, your nickname there,
  the roles you hold, when you joined, and notice when you join, leave, or any of that changes.
  This is used for automatic roles, welcome and goodbye messages, captcha verification, keeping a
  mute in place if you leave and rejoin, and moderation rules that check the name someone joins
  with.
- **Your online status and what you are doing**, including a game you are playing or a stream you
  have started. This is used only to announce when a member of the server starts a livestream and
  to give them a role while they are live, and to show your current status if a moderator looks you
  up.
- **The messages you send in that server**, including their text, attachments and embeds. This is
  used to run the server's automatic moderation rules, to save message logs, and to run custom
  commands that the server has set to respond to what members say.

Receiving information is not the same as keeping it. Most of what the bot sees is examined, acted
on, and immediately discarded. The next two sections set out exactly what is kept and what is
not.

## What we store

**Server settings.** Everything a server's administrators configure: which features are on, the
rules they have written, the channels and roles those features use, and the message templates they
have created. This includes the IDs of members who have been exempted from a rule or granted a
particular permission. Kept until the server's staff delete them. Removing the bot from a server
does not by itself erase its settings, so that a server which adds the bot again does not have to
set everything up from scratch.

**Moderation records.** When staff warn, mute, ban, or time out a member, we record who was acted
on, who did it, the reason given, and when. We also record punishments scheduled to end later, such
as a mute that expires in an hour, so we know when to lift them. Kept until the server's staff
delete them.

**Message logs.** We store the text of messages along with who wrote them and when, in three
cases. First, when a moderation action is taken against someone: if they are banned, kicked, muted
or warned, or if they are reported, we save a short snapshot of the surrounding conversation,
currently the last 100 messages in that channel, and attach it to the moderation record so that
staff reviewing the decision or handling an appeal can see the context. Second, when a moderator
saves a conversation on purpose in order to investigate something. Third, where a server has turned
on deleted-message logging. Storing the text is the purpose of the feature: it exists so staff can
still see what was said after a message has been deleted or edited. A server's excluded channels
are respected in all three cases. **Message logs are deleted automatically once they are 30 days
old**, whatever else happens. Staff can also delete them sooner.

**Support ticket records.** When a ticket is closed, the conversation is saved so staff have a
record, either posted into a channel the server chooses or attached as a text file. From that point
it lives in that server, under that server's control.

**Control panel sessions.** When you sign in at `<WEBSITE>` with your Discord account, we store a
session so you stay signed in, along with the access token Discord gives us to read which servers
you can manage. The session ends when you sign out or when it expires. We use cookies only for
signing in and for security, not for advertising or tracking you across other sites.

## What we do not store

**Member details.** Your roles, nickname, join date and similar details are held only in the bot's
temporary working memory so it can answer commands quickly. They are discarded once they have not
been used for a while, and none of it survives the bot restarting.

**Your status and activity.** What you are playing, listening to, or streaming is never written to
our database or to disk. It exists only in temporary memory and is discarded automatically. We do
not build profiles of what you do, we do not measure how long you spend in a game, and nothing in
the product shows your past activity. The one exception is a short list of who is currently
livestreaming, which exists purely so the same stream is not announced twice, and you are removed
from it the moment your stream ends.

**Message content, beyond the two features above.** Automatic moderation checks a message against a
server's rules and keeps no copy of it. If a rule is broken we record which rule and which member,
not what was said. Custom commands, command handling, reputation and timezone conversion all read
a message, act on it, and discard it.

## Who else receives information

We do not sell your information, share it for advertising or marketing, or give it to anyone else,
except:

- **Scam and phishing link checking.** Where a server has enabled it, web addresses found in
  messages are checked against SinkingYachts, BitFlow and Google Safe Browsing so they can tell us
  whether a link is dangerous. Only the domain or address is sent. No message text, no information
  about who wrote it, and no information about which server it came from.
- **Our hosting providers**, who hold the servers and databases this service runs on, in
  `<COUNTRY OR REGION>`.
- **Where the law requires it**, or where it is necessary to protect the safety of our users.

## How to have your information deleted

- **Server staff** can delete message logs, moderation records and ticket records for their server
  at any time through the control panel or the bot's commands.
- **Removing the bot** from a server does not by itself delete anything, so that a server which
  adds it again keeps its configuration.
- **Server owners and individual members** can contact us at `<CONTACT EMAIL>` about data we hold.
  Include your Discord user ID, or your server's ID, so that we can identify what you mean.

Some limits are worth stating plainly. Where a record belongs to a server rather than to an
individual, such as a warning issued by that server's staff, it is that server's moderation
history, and requests about it are generally a matter for that server's administrators. Ticket
records already posted into a server's channel are under that server's control, not ours.

## Your rights

Depending on where you live, you may have the right to know what we hold about you, to have it
corrected or deleted, to object to how we use it, and to receive a copy. Contact
`<CONTACT EMAIL>` to exercise any of these. You also have the right to complain to your local data
protection authority.

## Security

We restrict access to stored data to the people who operate the service. Saved message logs are
additionally protected inside each server: administrators choose which roles may read them, and
whether deleted messages are visible to anyone who is not a moderator. No system is perfectly
secure, but we take reasonable steps to protect what we hold.

## Changes

We may update this policy. The date at the top tells you when it last changed, and we will announce
significant changes in our support server.

---

# Optional sections

Each of these covers a feature that is off by default. If your instance has it switched on, paste
the section into the policy above and add the matching row to the summary table. If not, leave it
out. **Do not paste in a section describing something you do not do.**

## If you record username and nickname history

> **Name history.** If a member changes their username or their nickname in a server, we record the
> previous one, so that moderators can tell whether an account has recently changed name to avoid
> being recognised. Server administrators can switch this off for their server. Members can ask us
> to delete their name history at `<CONTACT EMAIL>`.

Add to the table: `Username and nickname history | Yes | Until deleted`

## If you record server statistics

> **Server statistics.** We count how many members join and leave a server, and how many messages
> are sent in it, so administrators can see a graph of how their community is changing over time.
> These are totals only. We do not keep a record of which member joined, left, or sent a message.

Add to the table: `Membership and message totals | Yes, as numbers only | Rolling window`

## If member verification records IP addresses

> **Verification records.** Where a server uses the captcha verification feature, we record that
> you completed it and the IP address you completed it from. This is used to detect one person
> creating many accounts to evade a ban. Contact `<CONTACT EMAIL>` about records held about you.

Add to the table: `Verification records, including IP address | Yes | <HOW LONG>`

## If you accept payment for premium features

> **Payments.** Payments are handled by `<PAYMENT PROVIDER>`, who processes your payment details.
> We never see or store your card details. We store which Discord account is entitled to premium
> features and which servers it has applied them to.

Add to the table: `Premium entitlements | Yes | While the subscription is active`

## If you run usage analytics

> **Usage analytics.** We count how often each feature is used, per server, so we know which parts
> of the bot are worth maintaining. These are counts attached to a server, never to a person.
