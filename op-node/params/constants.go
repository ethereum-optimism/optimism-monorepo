package params

const (
	// ChannelTimeoutGranite is a post-Granite constant: Number of L1 blocks between when a channel can be opened and when it must be closed by.
	ChannelTimeoutGranite uint64 = 50
	// MessageExpiryTimeSecondsInterop is a post-Interop constant for the minimum age of a message before it can be considered executable
	MessageExpiryTimeSecondsInterop = 15552000
)
