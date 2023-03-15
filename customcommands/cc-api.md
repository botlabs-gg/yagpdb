# Custom Commands Management API

This file documents a proposed API to manage custom commands. It serves to track progress and discuss details.

Later it should serve as documentation for developers who wish to integrate this API into their project, such as an
editor plugin or a CLI.

## General

The proposed API will be a RESTful HTTP(S) API.

Edits via the API *must* appear on the control panel logs, just like it is currently the case with edits from the
control panel.

Whether access to this API is to be locked behind YAGPDB Premium is up for discussion, though currently out of scope
of this proposal.

## Authorisation

Authorisation will be handled via Personal Access Tokens (PATs), which can be obtained from the Control Panel.
PATs will be tied to a specific user, granting them access to all servers they have edit access to. These PATs *must*
expire after a certain timespan. This timespan is up for discussion, notwithstanding a "never expire" option.

Clients are responsible to keep their personal token secret. In case a client does leak their token, one must be able to
easily reset it.

As for the specific implementation, [JSON Web Tokens (JWTs)](https://jwt.io/introduction/) appear to be the most
convenient way -- the specification explains that a JWT already holds some data that is part of the payload,
like the User ID the token is tied to.

Finally, the bearer token is to be sent in the standardised `Authorization` HTTP header.

Any request made without providing a valid token will result in an instant `401 Unauthorised`.

## Endpoints

There are some endpoints required to make this proposal worthwhile, and some optional ("nice to have") endpoints.
Optional endpoints will be marked with a "?" at the end of the respective URL.

This section is not to be understood as final -- endpoints can change as discussion progresses.

Any endpoint should return `403 Forbidden` if lacking access, `404 Not Found` if the requested resource (guilds also
count for this matter) does not exist. Consider always returning `404` to avoid leaking any active guilds.

### Creating a New Custom Command

`POST /api/:SERVERID/customcommands/new`?

- Accepts an optional [custom command object](#custom-command-object) as request payload.
  + Returns `204 No Content` on success.

If no payload was provided:
- Creates default custom command (just like on the control panel).
  + Returns `200 OK` with custom command object in response payload.

### Retrieving All Custom Commands

`GET /api/:SERVERID/customcommands`?

- Returns an array of all custom commands the requested server has, irregardless of whether they are enabled or not.
  + Returns `200 OK` with above response payload.

### Retrieving a Custom Command

`GET /api/:SERVERID/customcommands/:CCID`

- Returns `200 OK` with [custom command object](#custom-command-object) as response payload.

### Editing a Custom Command

`PATCH /api/:SERVERID/customcommands/:CCID`

- Edit custom command with `CCID`.
- Accepts [custom command object](#custom-command-object) in request payload.
- Empty payload edits it to default response.
- Returns `204 No Content` on success.
- Returns `400 Bad Request` when the sent code is longer than 10 000 characters. (Maybe `413 Payload Too Long`?)
- Returns `400 Bad Request` when the parser encounters an error. This error is transmitted in response payload.

### Deleting a Custom Command

`DELETE /api/:SERVERID/customcommands/:CCID`?

- Delete custom command with `CCID`.
- Returns `204 No Content` on success.

## Objects / Structs

As the proposed API ist a RESTful API, all objects are to be understood as a JSON object sent in either response or
request payload.

### Custom Command Object

The API will use the `CustomCommand` struct defined in [customcommands.go](customcommands.go).

The server will answer with a `400 Bad Request` if changes are made that are also not possible via the control panel,
such as changing the custom command ID, or providing an invalid interval range.

## Ratelimits

Currently, there will be no rate-limiting of requests, as the control panel is also not rate-limited.
