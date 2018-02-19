You can create custom embeds (messages with colored sidebar) with YAGPDB:
You can give the bot JSON input like the following: https://discordapp.com/developers/docs/resources/channel#embed-object.

You may also use https://leovoel.github.io/embed-visualizer/. But remember to remove the following part of the output:

```json{
  "content": "this `supports` __a__ **subset** *of* ~~markdown~~ 😃 ```js\nfunction foo(bar) {\n  console.log(bar);\n}\n\nfoo(1);```",
  "embed": {
```
Also remove the last } so that it matches the JSON syntax again. Your end result should look like the following:
```json{

    "title": "My title",
    "description": "My description",
    "url": "https://discordapp.com",
    "color": 4525733,
    "timestamp": "9999-12-31T23:59:59.708Z",
    "footer": {
      "icon_url": "https://cdn.discordapp.com/embed/avatars/0.png",
      "text": "My footer"
    },
    "thumbnail": {
      "url": "https://cdn.discordapp.com/embed/avatars/0.png"
    },
    "image": {
      "url": "https://cdn.discordapp.com/embed/avatars/0.png"
    },
    "author": {
      "name": "Author",
      "icon_url": "https://cdn.discordapp.com/embed/avatars/0.png"
    },
    "fields": [
      {
        "name": "Field one:",
        "value": "This is field one."
      },
      {
        "name": "Field two:",
        "value": "This is field two."
      }
    ]
  }
```
End result: 
![customembed.PNG]({{static "customembed.PNG"}})
