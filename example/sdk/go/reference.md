# Reference
## Meta
<details><summary><code>client.Meta.Hello() -> *notes.HelloMetaResponse</code></summary>
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
<details><summary><code>client.Notes.List() -> *notes.ListNotesResponse</code></summary>
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
request := &notes.CreateNotesRequest{
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

<details><summary><code>client.Notes.Delete(ID) -> error</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```go
request := &notes.DeleteNotesRequest{
    ID: int64(42),
}
client.Notes.Delete(
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

**id:** `int64` 
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

## People
<details><summary><code>client.People.List() -> *notes.ListPeopleResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```go
client.People.List(
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

<details><summary><code>client.People.SetRole(ID, request) -> *notes.Person</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```go
request := &notes.SetRolePeopleRequest{
    ID: "4f1c9a0e7b2d4c6f8a1b3c5d7e9f0a1b",
    Role: notes.SetRolePeopleRequestRoleMember,
}
client.People.SetRole(
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

**id:** `string` 
    
</dd>
</dl>

<dl>
<dd>

**role:** `notes.SetRolePeopleRequestRole` 
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

