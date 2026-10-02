# Reference
## Meta
<details><summary><code>client.Meta.Hello() -> *orpcapi.HelloOutputBody</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```go
client.Meta.Hello(
    context.TODO(),
)
```
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

## Notes
<details><summary><code>client.Notes.List() -> *orpcapi.ListOutputBody</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```go
request := &orpcapi.ListNotesRequest{
    Cursor: orpcapi.String(
        "42",
    ),
    Limit: orpcapi.Int(
        20,
    ),
}
client.Notes.List(
    context.TODO(),
    request,
)
```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**cursor:** `*string` — Opaque cursor from the previous page's next_cursor
    
</dd>
</dl>

<dl>
<dd>

**limit:** `*int` 
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.Notes.Create(request) -> *orpcapi.Note</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```go
request := &orpcapi.CreateInputBody{
    Body: "Buy milk",
}
client.Notes.Create(
    context.TODO(),
    request,
)
```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**body:** `string` 
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.Notes.Watch() -> orpcapi.Note</code></summary>
<dl>
<dd>

#### 📝 Description

<dl>
<dd>

<dl>
<dd>

Each event's SSE id is the note id, so a browser EventSource resumes by itself (Last-Event-ID).
</dd>
</dl>
</dd>
</dl>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```go
request := &orpcapi.WatchNotesRequest{
    After: orpcapi.String(
        "42",
    ),
    Seconds: orpcapi.Int(
        30,
    ),
}
client.Notes.Watch(
    context.TODO(),
    request,
)
```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**after:** `*string` — Resume after this note id (the id of the last note you received). Absent: only notes created from now on
    
</dd>
</dl>

<dl>
<dd>

**seconds:** `*int` — How long to keep the stream open
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

