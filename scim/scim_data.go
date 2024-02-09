package scim

type ICrmDataSource interface {
	Users(func(*User))
	Groups(func(*Group))
	Populate() error
}

type SyncStat struct {
	SuccessUsers      []string
	FailedUsers       []string
	SuccessGroups     []string
	FailedGroups      []string
	SuccessMembership []string
	FailedMembership  []string
}
type IScimSync interface {
	Source() ICrmDataSource
	Sync() (*SyncStat, error)
	Verbose() bool
	SetVerbose(bool)
	Destructive() bool
	SetDestructive(bool)
}

type User struct {
	Id        string
	Email     string
	FullName  string
	FirstName string
	LastName  string
	Active    bool
	Groups    []string
}

type Group struct {
	Id   string
	Name string
}

type ScimEndpointParameters struct {
	Url         string
	Token       string
	Verbose     bool
	Destructive bool
}

type GoogleEndpointParameters struct {
	AdminAccount string
	Credentials  []byte
	ScimGroups   []string
}
