package pages

import (
	"strconv"

	"github.com/gsxhq/gsx"

	"github.com/joeblew999/charter/examples/start-datastar/api"
)

// Home is the page at /: the form, and the newest messages, newest first. The list follows the
// stream from the newest message it shows (after): Datastar's @get opens it when the list appears
// (data-init), and each datastar-patch-elements event puts its message at the top of the list.
// retry 'always' connects again when a stream ends, with the id of the last event it had
// (Last-Event-ID), so no message is lost or shown twice.
component Home(messages []api.Message, after int64) {
	<Layout title="charter-start-datastar">
		<h1>Messages</h1>
		<p>Post one, and it shows at once on every page open here.</p>
		<Form body="" problem=""/>
		<ul id="messages" data-init={follows(after)}>
			{ for _, message := range messages {
				<Item message={message}/>
			} }
		</ul>
	</Layout>
}

// follows is the list's data-init. gsx cannot check a hole in a Datastar expression (an action
// starts with @, which is not JavaScript), so it is built here, from a number and nothing else.
func follows(after int64) gsx.RawJS {
	return gsx.RawJS("@get('/messages/stream?after=" + strconv.FormatInt(after, 10) + "', {retry: 'always'})")
}

// Form is the form, and what POST /messages answers: Datastar puts it in place of the one sent (its
// id, and the Datastar-Mode header: replace), empty, or with what was typed and why it was refused.
component Form(body string, problem string) {
	<form id="post" data-on:submit=js`@post('/messages', {contentType: 'form'})`>
		<input
			name="body"
			value={body}
			maxlength={strconv.Itoa(api.MaxBody)}
			required
			autofocus
			autocomplete="off"
			placeholder="Say something"
			aria-label="Message"
		/>
		<button type="submit">Post</button>
		{ if problem != "" {
			<p class="problem">{problem}</p>
		} }
	</form>
}

// Item is one message: in the list, and what the stream sends of each new one.
component Item(message api.Message) {
	<li id={"message-" + strconv.FormatInt(message.ID, 10)}>
		<span>{message.Body}</span>
		<time>{message.CreatedAt}</time>
	</li>
}
