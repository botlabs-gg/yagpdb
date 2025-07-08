# Authentication
The Opencloud APIs will let you authenticate either with an API key that you generated for your experience or with an OAuth token granted by a user authorizing with your application. Goblox supports both of these methods of authentication.

::: tip
Make sure that your API key or OAuth application has the proper scopes set. You can find the scopes in the [OpenCloud API reference](https://create.roblox.com/docs/en-us/cloud) or under the specific method you're calling.
:::

## API Keys
You can create a new API key at: https://create.roblox.com/dashboard/credentials?activeTab=ApiKeysTab
```go
package main

import (
    "context"
    "fmt"

    "github.com/typical-developers/goblox/opencloud"
)

func main() {
    ctx := context.Background()
    client := opencloud.NewClient().WithAPIKey("YOUR_API_KEY")

    user, resp, err := client.UserAndGroups.GetUser(ctx, "USER_ID")
    if err != nil {
        panic(err)
    }

    fmt.Println(fmt.Sprintf("User: %+v", user.Name))
}
```

## OAuth Tokens
You can create a new OAuth application at: https://create.roblox.com/dashboard/credentials?activeTab=OAuthTab
```go
package main

import (
    "context"
    "fmt"

    "github.com/typical-developers/goblox/opencloud"
)

var (
    client = opencloud.NewClient()
)

func main() {
    ctx := context.Background()
    authedClient := client.WithOAuthToken("YOUR_OAUTH_TOKEN")

    user, resp, err := authedClient.UserAndGroups.GetUser(ctx, "USER_ID")
    if err != nil {
        panic(err)
    }

    fmt.Println(fmt.Sprintf("User: %+v", user.Name))
}
```