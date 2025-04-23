# supabase-go

> **This fork adds Realtime support to the original repository.** The original project was [looking for a new home](https://github.com/nedpals/supabase-go/issues/67).

Unofficial [Supabase](https://supabase.io) client for Go. It is an amalgamation of all the libraries similar to the [official Supabase client](https://supabase.io/docs/reference/javascript/supabase-client).

## Installation
```
go get github.com/the-muppet/supabase-go
```

## Usage

Replace the `<SUPABASE-URL>` and `<SUPABASE-KEY>` placeholders with values from `https://supabase.com/dashboard/project/YOUR_PROJECT/settings/api`

### Authenticate

```go
package main
import (
    "fmt"
    "context"
    
    supa "github.com/the-muppet/supabase-go"
)

func main() {
  supabaseUrl := "<SUPABASE-URL>"
  supabaseKey := "<SUPABASE-KEY>"
  supabase := supa.CreateClient(supabaseUrl, supabaseKey)

  ctx := context.Background()
  user, err := supabase.Auth.SignUp(ctx, supa.UserCredentials{
    Email:    "example@example.com",
    Password: "password",
  })
  if err != nil {
    panic(err)
  }

  fmt.Println(user)
}
```

### Sign-In

```go
package main
import (
    "fmt"
    "context"
    
    supa "github.com/the-muppet/supabase-go"
)

func main() {
  supabaseUrl := "<SUPABASE-URL>"
  supabaseKey := "<SUPABASE-KEY>"
  supabase := supa.CreateClient(supabaseUrl, supabaseKey)

  ctx := context.Background()
  user, err := supabase.Auth.SignIn(ctx, supa.UserCredentials{
    Email:    "example@example.com",
    Password: "password",
  })
  if err != nil {
    panic(err)
  }

  fmt.Println(user)
}
```

### Insert

```go
package main
import (
    "fmt"
    
    supa "github.com/the-muppet/supabase-go"
)

type Country struct {
  ID      int    `json:"id"`
  Name    string `json:"name"`
  Capital string `json:"capital"`
}

func main() {
  supabaseUrl := "<SUPABASE-URL>"
  supabaseKey := "<SUPABASE-KEY>"
  supabase := supa.CreateClient(supabaseUrl, supabaseKey)

  row := Country{
    ID:      5,
    Name:    "Germany",
    Capital: "Berlin",
  }

  var results []Country
  err := supabase.DB.From("countries").Insert(row).Execute(&results)
  if err != nil {
    panic(err)
  }

  fmt.Println(results) // Inserted rows
}
```

### Select

```go
package main
import (
    "fmt"
    
    supa "github.com/the-muppet/supabase-go"
)

func main() {
  supabaseUrl := "<SUPABASE-URL>"
  supabaseKey := "<SUPABASE-KEY>"
  supabase := supa.CreateClient(supabaseUrl, supabaseKey)

  var results map[string]interface{}
  err := supabase.DB.From("countries").Select("*").Single().Execute(&results)
  if err != nil {
    panic(err)
  }

  fmt.Println(results) // Selected rows
}
```

### Update

```go
package main
import (
    "fmt"
    
    supa "github.com/the-muppet/supabase-go"
)

type Country struct {
  Name    string `json:"name"`
  Capital string `json:"capital"`
}

func main() {
  supabaseUrl := "<SUPABASE-URL>"
  supabaseKey := "<SUPABASE-KEY>"
  supabase := supa.CreateClient(supabaseUrl, supabaseKey)

  row := Country{
    Name:    "France",
    Capital: "Paris",
  }

  var results map[string]interface{}
  err := supabase.DB.From("countries").Update(row).Eq("id", "5").Execute(&results)
  if err != nil {
    panic(err)
  }

  fmt.Println(results) // Updated rows
}
```

### Delete

```go
package main
import (
    
    "fmt"

    supa "github.com/the-muppet/supabase-go"
)

func main() {
  supabaseUrl := "<SUPABASE-URL>"
  supabaseKey := "<SUPABASE-KEY>"
  supabase := supa.CreateClient(supabaseUrl, supabaseKey)

  var results map[string]interface{}
  err := supabase.DB.From("countries").Delete().Eq("name", "France").Execute(&results)
  if err != nil {
    panic(err)
  }

  fmt.Println(results) // Empty - nothing returned from delete
}
```

### Invite user by email

```go
package main
import (
    
    "fmt"
    "context"

    supa "github.com/the-muppet/supabase-go"
)

func main() {
  supabaseUrl := "<SUPABASE-URL>"
  supabaseKey := "<SUPABASE-KEY>"
  supabase := supa.CreateClient(supabaseUrl, supabaseKey)

  ctx := context.Background()
  user, err := supabase.Auth.InviteUserByEmail(ctx, email)
  if err != nil {
    panic(err)
  }

  // or if you want to setup some metadata
  data := map[string]interface{}{ "invitedBy": "someone" }
  redirectTo := "https://your_very_successful_app.com/signup"
  user, err = supabase.Auth.InviteUserByEmailWithData(ctx, email, data, redirectTo)
  if err != nil {
    panic(err)
  }

  fmt.Println(user)
}
```

### Subscribing to Database Changes

```go
package main

import (
   
    "fmt"

    supa "github.com/the-muppet/supabase-go"
)

func main() {
    supabaseUrl := "<SUPABASE-URL>"
    supabaseKey := "<SUPABASE-KEY>"
    supabase := supa.CreateClient(supabaseUrl, supabaseKey)

    channel := supabase.Channel("db-changes")
    
    channel.SubscribeToPostgresChanges([]supa.PostgresChange{
        {
            Event:  "*",       // Listen to all events (INSERT, UPDATE, DELETE)
            Schema: "public",  // Schema name
            Table:  "todos",   // Table name
        },
    })

    // Listen for changes
    err := channel.Subscribe(func(message supa.Message) {
        fmt.Println("Received message:", message)
        
        // Extract data from the payload
        payload, ok := message.Payload["record"].(map[string]interface{})
        if ok {
            fmt.Println("Changed record:", payload)
        }
    })
    
    if err != nil {
        panic(err)
    }
    
    // Connect to the Realtime server
    err = supabase.ConnectRealtime()
    if err != nil {
        panic(err)
    }
    
    // Join the channel
    err = channel.Join()
    if err != nil {
        panic(err)
    }
    
    // Wait for changes
    select {}
}
```

### Using Presence to Track Online Users

```go
package main

import (
    "fmt"

    supa "github.com/the-muppet/supabase-go"
)

func main() {
    supabaseUrl := "<SUPABASE-URL>"
    supabaseKey := "<SUPABASE-KEY>"
    supabase := supa.CreateClient(supabaseUrl, supabaseKey)
    
    channel := supabase.Channel("room:lobby")
    
    // Subscribe to presence
    channel.SubscribeToPresence("user-123")
    
    // Subscribe to presence changes
    err := channel.SubscribeToEvent("presence_state", func(message supa.Message) {
        fmt.Println("Presence state:", message.Payload)
        
        // Get the current presence state
        state := channel.GetPresenceState()
        fmt.Println("Current users:", state)
    })
    
    if err != nil {
        panic(err)
    }
    
    err = channel.SubscribeToEvent("presence_diff", func(message supa.Message) {
        fmt.Println("Presence diff:", message.Payload)
    })
    
    if err != nil {
        panic(err)
    }
    
    // Connect to the Realtime server
    err = supabase.ConnectRealtime()
    if err != nil {
        panic(err)
    }
    
    // Join the channel
    err = channel.Join()
    if err != nil {
        panic(err)
    }
    
    // Track the user's presence
    err = channel.Track(map[string]interface{}{
        "user_id": "user-123",
        "status": "online",
        "username": "johndoe",
    })
    
    if err != nil {
        panic(err)
    }
    
    // Wait for changes
    select {}
}
```

### Broadcasting Messages Between Clients

```go
package main

import (
  "fmt"

  supa "github.com/the-muppet/supabase-go"
)

func main() {
    supabaseUrl := "<SUPABASE-URL>"
    supabaseKey := "<SUPABASE-KEY>"
    supabase := supa.CreateClient(supabaseUrl, supabaseKey)
    
    // Create a channel
    channel := supabase.Channel("room:lobby")
    
    // Subscribe to broadcast
    channel.SubscribeToBroadcast([]string{"message"}, supa.BroadcastConfig{
        Self: true, // Receive your own messages
    })
    
    // Subscribe to messages
    err := channel.SubscribeToEvent("message", func(message supa.Message) {
        fmt.Println("Received message:", message.Payload)
    })
    
    if err != nil {
        panic(err)
    }
    
    // Connect to the Realtime server
    err = supabase.ConnectRealtime()
    if err != nil {
        panic(err)
    }
    
    // Join the channel
    err = channel.Join()
    if err != nil {
        panic(err)
    }
    
    // Broadcast a message
    err = channel.Broadcast("message", map[string]interface{}{
        "text": "Hello, world!",
        "user": "user-123",
    })
    
    if err != nil {
        panic(err)
    }
    
    // Wait for changes
    select {}
}
```

## API Reference

### Client Methods

- `client.Channel(name string)` - Creates a new Realtime channel
- `client.ChannelWithOptions(name string, options *supa.ChannelOptions)` - Creates a new channel with custom options
- `client.ConnectRealtime()` - Establishes the WebSocket connection
- `client.DisconnectRealtime()` - Closes the WebSocket connection
- `client.SetRealtimeAuth(token string)` - Sets the authentication token
- `client.GetRealtimeStatus()` - Returns the current connection status

### Channel Methods

- `channel.Subscribe(callback supa.EventHandler)` - Subscribes to all events on the channel
- `channel.SubscribeToEvent(event string, callback supa.EventHandler)` - Subscribes to a specific event
- `channel.Unsubscribe()` - Unsubscribes from the channel
- `channel.Join()` - Joins the channel
- `channel.Broadcast(event string, payload interface{})` - Broadcasts a message
- `channel.Track(payload interface{})` - Tracks presence
- `channel.Untrack()` - Untracks presence
- `channel.GetPresenceState()` - Returns the current presence state
- `channel.SubscribeToPostgresChanges(changes []supa.PostgresChange)` - Subscribes to PostgreSQL changes
- `channel.SubscribeToBroadcast(events []string, opts supa.BroadcastConfig)` - Subscribes to broadcast events
- `channel.SubscribeToPresence(key string)` - Subscribes to presence events
- `channel.Status()` - Returns the current status of the channel

## Roadmap

- [x] Auth support (1)
- [x] DB support (2)
- [x] Realtime
- [x] Storage
- [ ] Testing

(1) - Thin API wrapper. Does not rely on the GoTrue library for simplicity
(2) - Through `postgrest-go`

## Design Goals

It tries to mimick as much as possible the official Javascript client library in terms of ease-of-use and in setup process.

## Contributing

### Submitting a pull request

- Fork it (https://github.com/the-muppet/supabase-go/fork)
- Create your feature branch (git checkout -b my-new-feature)
- Commit your changes (git commit -am 'Add some feature')
- Push to the branch (git push origin my-new-feature)
- Create a new Pull Request

## Contributors

- [nedpals](https://github.com/nedpals) - creator and maintainer of the original repository
- [the-muppet](https://github.com/the-muppet) - realtime fork implementation