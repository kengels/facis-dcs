package archivestatus

type ArchiveStatus string

const (
	Stored  ArchiveStatus = "STORED"
	Deleted ArchiveStatus = "DELETED"
)

func (s ArchiveStatus) String() string {
	return string(s)
}
