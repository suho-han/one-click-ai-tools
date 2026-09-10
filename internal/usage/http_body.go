package usage

import "io"

// maxAPIBodyBytes caps provider API response reads. Legitimate payloads here
// are a few KB; capping keeps a misbehaving endpoint from buffering
// unbounded data.
const maxAPIBodyBytes = 2 << 20

// readAllCapped reads a response body with a size cap so huge or hostile
// responses are truncated instead of buffered in full.
func readAllCapped(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, maxAPIBodyBytes))
}
