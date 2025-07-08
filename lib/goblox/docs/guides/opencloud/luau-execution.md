# Luau Execution
The OpenCloud APIs allow you to execute Luau code in one of your experiences by spinning up a experience instance and executing the code.

### Executing Luau
The code below will create a new task operation for executing the code in the experience.
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

    task, _, err := client.LuauExecution.CreateLuauExecutionSessionTask(ctx, "UNIVERSE_ID", "PLACE_ID", nil, opencloud.LuauExecutionTaskCreate{
        Script: opencloud.Pointer("return 1 + 2"),
    })
    if err != nil {
        panic(err)
    }
}
```

However, this in itself is not very useful, since it runs asynchronously. We need a way to check when the task is finished and get the results or error from it. This is where polling the method comes in.

### Polling
Goblox provides a utility method, under [`methodutil`](/packages/methodutil), that can be used to poll a method until it is completed. The [`LuauExecutionTask.TaskInfo()`](/documentation/opencloud/luau-execution#taskinfo) method returns the task information, which we can use to easily reference the task creation information for polling.
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

Now we can keep checking the status of the task until it is completed and set our result or error into variables.