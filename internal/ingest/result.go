// internal/ingest/result.go
package ingest

type Status int

const (
	StatusInserted Status = iota
	StatusDuplicate
)

type Result struct {
	Status Status
}

func (r Result) Inserted() bool  { return r.Status == StatusInserted }
func (r Result) Duplicate() bool { return r.Status == StatusDuplicate }
