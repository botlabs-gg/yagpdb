# methodutil
The `methodutil` package contains utility methods for execution of other methods.

## `PollMethod`
This is a utility method that allows you to poll a method until it returns a desired result.

This is useful for endpoints that asynchronously run. A good example of this is the [LuauExecutionSessionTask](https://create.roblox.com/docs/en-us/cloud/reference/LuauExecutionSessionTask) API for asynchronously running Luau code in your experience.

Some APIs will return an [`Operation`](/documentation/opencloud/common#operation) result with a path instead that you can poll until it's complete.

### Example
In this example, we will run Luau code in our experience that gets the sum of 1 + 2.
```go
package main

import (
    "context"
    "fmt"

    "github.com/typical-developers/goblox/opencloud"
    "github.com/typical-developers/goblox/pkg/methodutil"
)

func main() {
    ctx := context.Background()
    client := opencloud.NewClient().WithAPIKey("YOUR_API_KEY")

    var result int
    var taskError error

    // First, we create the task with the Luau execution API.
    task, _, err := client.LuauExecution.CreateLuauExecutionSessionTask(ctx, "UNIVERSE_ID", "PLACE_ID", nil, opencloud.LuauExecutionTaskCreate{
        Script: opencloud.Pointer("return 1 + 2"),
    })
    if err != nil {
        panic(err)
    }

    // Then, we get the TaskInfo so we can get the task we just created.
    universeId, placeId, versionId, sessionId, taskId := task.TaskInfo()

    // Finally, we poll the task until it's complete and set the reuslt / error in our variables above.
	methodutil.PollMethod(func(done func()) {
		task, resp, err := client.LuauExecution.GetLuauExecutionSessionTask(ctx, universeId, placeId, versionId, sessionId, taskId)
        if err != nil {
            taskError = err
            done()
            return
        }

        // Keeps polling if the ratelimit is exhausted.
        if resp.StatusCode == 429 {
            return
        }

        // Queued means the task is still pending to be executed.
        // Processing means the task is currently being executed.
        // 
        // Only handle the data if the task is done being executed.
        if task.State != opencloud.LuauExecutionStateProcessing && task.State != opencloud.LuauExecutionStateQueued {
            if task.Output != nil && len(task.Output.Results) > 0 {
                result = task.Output.Results[0].(int)
            }

            if task.Error != nil {
                taskError = fmt.Errorf("LuauExecutionTask[%s]: %s", task.Error.Code, task.Error.Message)
            }

            done()
        }
	}, 0)

    fmt.Printf("Result: %d\n", result)
    fmt.Printf("Error: %v\n", taskError)
}
```