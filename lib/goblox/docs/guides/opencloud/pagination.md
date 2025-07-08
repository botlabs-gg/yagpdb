# Pagination
OpenCloud APIs that return an array of results support pagination.

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

    options := &opencloud.Options{
        MaxPageSize: opencloud.Pointer(100),
    }

    var inventory []*opencloud.InventoryItem
    for {
        items, resp, err := client.UsersAndGroups.ListInventoryItems(ctx, "USER_ID", options)
        if err != nil {
            return
        }

        inventory = append(inventory, items.InventoryItems...)
        if !items.NextPageToken {
            break
        }

        options.PageToken = opencloud.Pointer(items.NextPageToken)
    }
}
```