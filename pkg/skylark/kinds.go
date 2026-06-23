package skylark

// Knowledge kinds for vault index and session search.
const (
	KindDocument = "document" // vault markdown chunk
	KindHistory  = "history"  // conversation turn (session_search only, not in Bleve)
)
