# About <img src="/goblox_icon.png" align="right" width="100" />
Goblox is a Go library that makes accessing Roblox's OpenCloud & Legacy APIs with your Go projects easy. The library supports *all* OpenCloud APIs. Legacy (cookie) APIs will be added in the future.

::: warning
This library is still in development and should not be considered stable. If you want to contribute, feel free to open a PR!
:::

## Installation
::: info
Go 1.18 or newer is required.
:::

```bash
go get -u github.com/typical-developers/goblox
```

## Basic Usage
```go
package main

import (
    "context"
    "fmt"

    "github.com/typical-developers/goblox/opencloud"
)

func main() {
    client := opencloud.NewClient().WithAPIKey("YOUR_API_KEY")
}
```