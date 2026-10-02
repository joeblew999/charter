# Reference
## Meta
<details><summary><code>client.Meta.Hello() -> *notes.HelloOutputBody</code></summary>
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
<details><summary><code>client.Notes.List() -> *notes.ListOutputBody</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```go
request := &notes.ListNotesRequest{
    Cursor: notes.String(
        "42",
    ),
    Limit: notes.Int(
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

<details><summary><code>client.Notes.Create(request) -> *notes.Note</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```go
request := &notes.CreateInputBody{
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

<details><summary><code>client.Notes.Watch() -> notes.Note</code></summary>
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
request := &notes.WatchNotesRequest{
    After: notes.String(
        "42",
    ),
    Seconds: notes.Int(
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

