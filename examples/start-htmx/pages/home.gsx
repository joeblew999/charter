package pages

import (
	"strconv"

	"github.com/joeblew999/charter/examples/start-htmx/api"
)

// Home is the page at /: the form, and the newest messages, newest first. The list follows the
// stream from the newest message it shows (after): htmx 4's SSE extension (hx-sse:connect) puts the
// HTML of each event at the top of the list (hx-swap), and when it connects again it sends the id
// of the last event it had, so no message is lost or shown twice.
component Home(messages []api.Message, after int64) {
	<Layout title="charter-start-htmx">
		<h1>Messages</h1>
		<p>Post one, and it shows at once on every page open here.</p>
		<Form body="" problem=""/>
		<ul
			id="messages"
			hx-sse:connect={"/messages/stream?after=" + strconv.FormatInt(after, 10)}
			hx-swap="afterbegin"
		>
			{ for _, message := range messages {
				<Item message={message}/>
			} }
		</ul>
	</Layout>
}

// Form is the form, and what POST /messages answers: htmx puts it in place of the one sent
// (outerHTML), empty, or with what was typed and why it was refused.
component Form(body string, problem string) {
	<form id="post" hx-post="/messages" hx-swap="outerHTML">
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
