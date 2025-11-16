package team

type Member struct {
	ID       string
	Username string
	IsActive bool
}

type Team struct {
	Name    string
	Members []Member
}
