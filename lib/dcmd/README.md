# dcmd

dcmd is a extensible discord command system based on interfaces.

It's very much work in progress at the moment, if you start using it now you have to be okay with things changing and the fact that you will find bugs.

**Note** Only works with my fork of discordgo, if you want to use this with bwmarrin/discordgo with go modules use v1.0.0 which is the last version of this that supported that lib: `go get -u github.com/jonas747/dcmd@v1.0.0`

## Features:

For now look in the example folder. Still planning things out.

## TODO:

 - [ ] Full test coverage (See below for info on progress)
 - [ ] Only build middleware chains once?
      + [x] Added ability to prebuild middleware chains
      + [ ] Automatically do so   
 - [x] Standard Help generator

## Test Coverage:

 - Argument parsers
      + [x] int
      + [x] float
      + [ ] string
      + [ ] user
      + [x] Full line argdef parsing
      + [ ] Full line switch parsing
 - System
      + [x] FindPrefix
      + [ ] HandleResponse
 - Container
      + [ ] Middleware chaining
      + [ ] Add/Remove middleware
      + [ ] Command searching
 - Other
      + [ ] Help

### Time of day example

```go

func main() {
      // Create a new command system
      system := dcmd.NewStandardSystem("[")

      // Add the time of day command to the root container of the system
      system.Root.AddCommand(&CmdTimeOfDay{Format: time.RFC822}, dcmd.NewTrigger("Time", "t"))

      // Create the discordgo session
      session, err := discordgo.New(os.Getenv("DG_TOKEN"))
      if err != nil {
            log.Fatal("Failed setting up session:", err)
      }

      // Add the command system handler to discordgo
      session.AddHandler(system.HandleMessageCreate)

      err = session.Open()
      if err != nil {
            log.Fatal("Failed opening gateway connection:", err)
      }
      log.Println("Running, Ctrl-c to stop.")
      select {}
}

type CmdTimeOfDay struct {
      Format string
}

// Descriptions should return a short description (used in the overall help overiview) and one long descriptions for targetted help
func (t *CmdTimeOfDay) Descriptions(d *dcmd.Data) (string, string) {
      return "Responds with the current time in utc", ""
}

// Run implements the dcmd.Cmd interface and gets called when the command is invoked
func (t *CmdTimeOfDay) Run(data *dcmd.Data) (interface{}, error) {
      return time.Now().UTC().Format(t.Format), nil
}


```
