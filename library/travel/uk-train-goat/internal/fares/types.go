package fares

type Location struct {
	NLC, CRS, Name     string
	StartDate, EndDate string // YYYYMMDD
}
type Flow struct {
	FlowID, OriginNLC, DestNLC, Route, Direction, TOC string
	StartDate, EndDate                                string
}
type Fare struct {
	FlowID, TicketCode string
	Pence              int
	RestrictionCode    string
}
type NonDerivableFare struct {
	OriginNLC, DestNLC, Route, TicketCode, RestrictionCode string
	Pence                                                  int
	StartDate, EndDate                                     string
}
type TicketType struct {
	Code, Description, TicketClass, TicketType string // TicketType: S=single R=return
}
type Railcard struct {
	Code, Description string
	MinPence          int
	DiscountPct       int // adult-fare discount, whole percent
}
type ClusterMember struct{ ClusterID, MemberNLC, StartDate, EndDate string }
type GroupMember struct {
	MemberNLC string
	GroupNLC  string
	EndDate   string // YYYYMMDD (parseDate of the ddmmyyyy feed value)
}
type Restriction struct{ Code, Description string }
type FeedMeta struct {
	Sequence, LastModified, PublishDate, SyncedAt string
}
