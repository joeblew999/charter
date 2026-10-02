module soak

go 1.27.1

require github.com/joeblew999/charter/examples/notes-go/sdk/go v0.0.0

require github.com/google/uuid v1.6.0 // indirect

// The committed Go SDK. test/soak.mjs builds with a workspace file that points at a generated one instead.
replace github.com/joeblew999/charter/examples/notes-go/sdk/go => ../../sdk/go
